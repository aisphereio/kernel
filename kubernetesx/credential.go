package kubernetesx

import (
	"errors"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// CredentialKind identifies how a cluster is authenticated.
type CredentialKind string

const (
	// CredentialKindKubeconfig authenticates via a kubeconfig byte blob.
	// The kubeconfig must not use an exec plugin or reference external
	// files; see Validate.
	CredentialKindKubeconfig CredentialKind = "KUBECONFIG"

	// CredentialKindInCluster authenticates via the in-cluster service
	// account mounted in the running pod. Only valid when Hub itself runs
	// on the target cluster.
	CredentialKindInCluster CredentialKind = "IN_CLUSTER"

	// CredentialKindServiceAccount authenticates with a long-lived
	// service-account token and an explicit Host and optional CA cert.
	CredentialKindServiceAccount CredentialKind = "SERVICE_ACCOUNT"
)

// Credential is the input form Hub sends to kubernetesx when building a
// per-cluster Client. It carries secret material and must never be logged or
// serialized into API responses.
type Credential struct {
	// Kind selects the authentication strategy. Required.
	Kind CredentialKind `json:"kind"`

	// Kubeconfig is the raw kubeconfig YAML. Required when Kind ==
	// KUBECONFIG. Ignored otherwise.
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// Context selects the kubeconfig context. Optional; empty means the
	// kubeconfig current-context.
	Context string `json:"context"`

	// Host is the API server URL. Required when Kind == SERVICE_ACCOUNT.
	Host string `json:"host"`

	// Token is the bearer token. Required when Kind == SERVICE_ACCOUNT.
	Token string `json:"token"`

	// CACert is the PEM-encoded CA certificate that signs the API server
	// cert. Optional; when empty the system roots are used (or
	// InsecureSkipVerify from Config).
	CACert []byte `json:"ca_cert"`
}

// Validate enforces the security invariants in design §5:
//
//   - KUBECONFIG must parse and must not use an exec plugin or reference
//     external files (certificate paths outside the kubeconfig blob are
//     rejected because Hub does not ship those files to its environment);
//   - SERVICE_ACCOUNT must have Host and Token;
//   - IN_CLUSTER requires nothing (resolved at runtime from the pod).
//
// Validate does not touch the API server; it only sanity-checks the input.
func (c Credential) Validate() error {
	switch c.Kind {
	case CredentialKindInCluster:
		return nil
	case CredentialKindKubeconfig:
		if len(c.Kubeconfig) == 0 {
			return errors.New("kubernetesx: kubeconfig credential has empty kubeconfig")
		}
		return validateKubeconfig(c.Kubeconfig)
	case CredentialKindServiceAccount:
		if c.Host == "" {
			return errors.New("kubernetesx: service-account credential requires Host")
		}
		if c.Token == "" {
			return errors.New("kubernetesx: service-account credential requires Token")
		}
		return nil
	default:
		return ErrCredentialInvalid
	}
}

// validateKubeconfig loads the kubeconfig via clientcmd and rejects exec
// plugins and external file references. An exec plugin would let a kubeconfig
// run arbitrary local commands; an external file reference would silently
// break when Hub lacks that file. Both are forbidden for cluster onboarding.
func validateKubeconfig(raw []byte) error {
	cfg, err := clientcmd.Load(raw)
	if err != nil {
		return err
	}
	if err := clientcmd.Validate(*cfg); err != nil {
		return err
	}
	for name, auth := range cfg.AuthInfos {
		if auth == nil {
			continue
		}
		if auth.Exec != nil {
			return errors.New("kubernetesx: kubeconfig user " + name + " uses exec plugin (forbidden)")
		}
		if auth.ClientCertificate != "" || auth.ClientKey != "" {
			return errors.New("kubernetesx: kubeconfig user " + name + " references external cert files (forbidden)")
		}
		if auth.TokenFile != "" {
			return errors.New("kubernetesx: kubeconfig user " + name + " references external token file (forbidden)")
		}
		if auth.Impersonate != "" {
			return errors.New("kubernetesx: kubeconfig user " + name + " impersonates (forbidden)")
		}
	}
	for name, cluster := range cfg.Clusters {
		if cluster == nil {
			continue
		}
		if cluster.CertificateAuthority != "" {
			return errors.New("kubernetesx: kubeconfig cluster " + name + " references external CA file (forbidden)")
		}
	}
	// Reject path-based location-of-origin tricks: a fully self-contained
	// kubeconfig blob has no external dependencies, which is what Hub stores.
	if strings.Contains(string(raw), "file://") {
		return errors.New("kubernetesx: kubeconfig references file:// URI (forbidden)")
	}
	return nil
}

// hasExternalFileRef is a small helper exposed for tests that want to assert
// the rejection path without building a full clientcmdapi.Config.
func hasExternalFileRef(raw []byte) bool {
	return strings.Contains(string(raw), "file://")
}

// ensure clientcmdapi import is used for documentation cross-reference even
// if future refactor removes the direct type usage.
var _ = clientcmdapi.Config{}
