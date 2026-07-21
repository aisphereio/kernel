package kubernetesx

import (
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aisphereio/kernel/logx"
)

// New builds a kubernetesx.Client from cfg and opts. The construction order:
//
//  1. Validate cfg.
//  2. Resolve *rest.Config from Host / Kubeconfig / in-cluster, applying QPS,
//     Burst, Timeout, UserAgent, and InsecureSkipVerify.
//  3. Build the runtime.Scheme (WithScheme or DefaultScheme, plus any
//     WithAddToScheme additions).
//  4. Build the controller-runtime client, dynamic.Interface, and discovery
//     client from the *rest.Config (or use injected test doubles).
//  5. Register metrics.
//
// New never contacts the API server; reachability is checked separately via
// Probe. This keeps client construction fast and side-effect-free.
func New(cfg Config, opts ...Option) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, NormalizeError(err)
	}
	cfg = cfg.Normalized()

	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	logger := o.logger
	if logger == nil {
		logger = cfg.Logger
	}
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	logger = logger.Named("kubernetesx")

	if o.metricsEnabled {
		cfg.MetricsEnabled = true
	}
	if o.metrics != nil {
		cfg.Metrics = o.metrics
	}
	registerMetrics(cfg)

	restCfg, err := resolveRESTConfig(cfg, o)
	if err != nil {
		return nil, err
	}

	scheme, err := resolveScheme(cfg, o)
	if err != nil {
		return nil, err
	}

	raw, err := ctrlclient.New(restCfg, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		return nil, NormalizeError(err)
	}

	dyn := o.dynamic
	if dyn == nil {
		dyn, err = dynamic.NewForConfig(restCfg)
		if err != nil {
			return nil, NormalizeError(err)
		}
	}

	disco := o.discovery
	if disco == nil {
		cs, derr := kubernetes.NewForConfig(restCfg)
		if derr != nil {
			return nil, NormalizeError(derr)
		}
		disco = cs.Discovery()
	}

	return &client{
		raw:          raw,
		restConfig:   restCfg,
		scheme:       scheme,
		dyn:          dyn,
		disco:        disco,
		fieldManager: cfg.FieldManager,
		logger:       logger,
	}, nil
}

// resolveRESTConfig builds the *rest.Config from the supplied Config and
// options. When a test injects *rest.Config via WithRESTConfig it wins.
func resolveRESTConfig(cfg Config, o options) (*rest.Config, error) {
	if o.restConfig != nil {
		return applyRESTDefaults(o.restConfig, cfg), nil
	}

	if len(cfg.Kubeconfig) > 0 {
		restCfg, err := kubeconfigToRESTConfig(cfg.Kubeconfig, cfg.Context)
		if err != nil {
			return nil, NormalizeError(err)
		}
		return applyRESTDefaults(restCfg, cfg), nil
	}

	if cfg.Host != "" {
		restCfg := &rest.Config{Host: cfg.Host}
		if cfg.InsecureSkipVerify {
			restCfg.TLSClientConfig.Insecure = true
		}
		// Service-account credential: apply bearer token, CA cert and
		// ServerName. Without these the client would be unauthenticated and
		// (when Host is an IP) TLS would fail or skip verification. See
		// Config.MergeCredential ServiceAccount branch.
		if cfg.Token != "" {
			restCfg.BearerToken = cfg.Token
		}
		if len(cfg.CACert) > 0 {
			restCfg.CAData = cfg.CACert
		}
		if cfg.ServerName != "" {
			restCfg.TLSClientConfig.ServerName = cfg.ServerName
		}
		return applyRESTDefaults(restCfg, cfg), nil
	}

	// In-cluster fallback.
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, NormalizeError(err)
	}
	return applyRESTDefaults(restCfg, cfg), nil
}

// kubeconfigToRESTConfig parses a kubeconfig byte blob and selects the named
// context (or current-context when empty), returning a *rest.Config.
func kubeconfigToRESTConfig(raw []byte, contextName string) (*rest.Config, error) {
	loader := clientcmd.NewDefaultClientConfig(
		clientcmdapi.Config{},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{}, CurrentContext: contextName},
	)
	// Load directly from bytes via clientcmd.Load so we never touch the
	// filesystem (Hub stores self-contained kubeconfigs only; see
	// Credential.Validate).
	cfg, err := clientcmd.Load(raw)
	if err != nil {
		return nil, err
	}
	if err := clientcmd.Validate(*cfg); err != nil {
		return nil, err
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	loader = clientcmd.NewNonInteractiveClientConfig(*cfg, overrides.CurrentContext, overrides, nil)
	return loader.ClientConfig()
}

// applyRESTDefaults stamps QPS/Burst/Timeout/UserAgent/UserAgent from Config
// onto a *rest.Config. Values left at zero keep client-go defaults.
func applyRESTDefaults(restCfg *rest.Config, cfg Config) *rest.Config {
	if cfg.QPS > 0 {
		restCfg.QPS = cfg.QPS
	}
	if cfg.Burst > 0 {
		restCfg.Burst = cfg.Burst
	}
	if cfg.Timeout > 0 {
		restCfg.Timeout = cfg.Timeout
	}
	if cfg.UserAgent != "" {
		restCfg.UserAgent = cfg.UserAgent
	}
	if cfg.ServerName != "" {
		// Preserve CA verification: only override the expected TLS hostname,
		// do not disable verification. This lets Hub reach a cluster via an
		// IP while the server cert is signed for an in-cluster DNS name.
		restCfg.TLSClientConfig.ServerName = cfg.ServerName
	}
	if cfg.InsecureSkipVerify {
		restCfg.TLSClientConfig.Insecure = true
		// When Insecure is set, CA data must not be forced; client-go
		// ignores CAFile/CAData in that mode anyway.
		restCfg.TLSClientConfig.CAFile = ""
		restCfg.TLSClientConfig.CAData = nil
	}
	return restCfg
}

// resolveScheme picks the base scheme (WithScheme or DefaultScheme) and layers
// any WithAddToScheme additions.
func resolveScheme(cfg Config, o options) (*runtime.Scheme, error) {
	scheme := o.scheme
	if scheme == nil {
		scheme = DefaultScheme()
	}
	for _, add := range o.extraSchemes {
		if err := mergeSchemes(scheme, add); err != nil {
			return nil, NormalizeError(err)
		}
	}
	return scheme, nil
}

// ParseHost strips a trailing scheme marker from a Host string for logging.
// It is intentionally permissive: it only lowercases for comparison.
func ParseHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

// ensure imports are anchored for future direct usage (discovery/dynamic are
// exposed via the Client escape hatches; clientcmdapi underpins kubeconfig
// parsing).
var (
	_ discovery.DiscoveryInterface
	_ dynamic.Interface
	_ clientcmdapi.Config
	_ = errors.New
)
