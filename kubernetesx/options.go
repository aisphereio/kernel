package kubernetesx

import (
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Option configures a kubernetesx Client at construction time.
type Option func(*options)

type options struct {
	scheme         *runtime.Scheme
	extraSchemes   []AddToScheme
	logger         logx.Logger
	metrics        metricsx.Manager
	metricsEnabled bool

	// Injectable escape-hatch clients, used by tests and by the fake client
	// builder. When nil the factory constructs them from *rest.Config.
	restConfig *rest.Config
	dynamic    dynamic.Interface
	discovery  discovery.DiscoveryInterface
}

// WithScheme sets the base runtime.Scheme used by the client. When supplied it
// replaces DefaultScheme; otherwise DefaultScheme is used. Additional
// third-party schemes can still be layered via WithAddToScheme.
func WithScheme(scheme *runtime.Scheme) Option {
	return func(o *options) { o.scheme = scheme }
}

// WithAddToScheme registers an additional scheme (e.g. a third-party CRD
// type) onto the client's scheme. Multiple WithAddToScheme options are
// applied in order.
func WithAddToScheme(add AddToScheme) Option {
	return func(o *options) { o.extraSchemes = append(o.extraSchemes, add) }
}

// WithLogger overrides the Config.Logger used by the client. If not set, the
// factory falls back to Config.Logger (and then logx.DefaultLogger()).
func WithLogger(logger logx.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// WithMetrics overrides the Config.Metrics manager used by the client.
func WithMetrics(m metricsx.Manager) Option {
	return func(o *options) { o.metrics = m }
}

// WithMetricsEnabled overrides Config.MetricsEnabled.
func WithMetricsEnabled(enabled bool) Option {
	return func(o *options) { o.metricsEnabled = enabled }
}

// WithRESTConfig injects a pre-built *rest.Config, bypassing kubeconfig/Host
// resolution. Intended for tests and envtest.
func WithRESTConfig(cfg *rest.Config) Option {
	return func(o *options) { o.restConfig = cfg }
}

// WithDynamicClient injects a pre-built dynamic.Interface. Intended for tests.
func WithDynamicClient(d dynamic.Interface) Option {
	return func(o *options) { o.dynamic = d }
}

// WithDiscoveryClient injects a pre-built discovery client. Intended for tests.
func WithDiscoveryClient(d discovery.DiscoveryInterface) Option {
	return func(o *options) { o.discovery = d }
}
