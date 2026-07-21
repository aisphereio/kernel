package kubernetesx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/kubernetesx"
	"github.com/aisphereio/kernel/kubernetesx/fake"
)

// -----------------------------------------------------------------------------
// Config
// -----------------------------------------------------------------------------

func TestConfigValidate_RejectsNegativeTimeout(t *testing.T) {
	if err := (kubernetesx.Config{Timeout: -1 * time.Second}).Validate(); !errors.Is(err, kubernetesx.ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestConfigValidate_RejectsNegativeQPSAndBurst(t *testing.T) {
	for _, cfg := range []kubernetesx.Config{
		{QPS: -1},
		{Burst: -1},
	} {
		if err := cfg.Validate(); !errors.Is(err, kubernetesx.ErrConfigInvalid) {
			t.Fatalf("expected ErrConfigInvalid for %+v, got %v", cfg, err)
		}
	}
}

func TestConfigValidate_AcceptsEmptyForInCluster(t *testing.T) {
	// An all-zero Config is valid: the factory falls back to in-cluster.
	if err := (kubernetesx.Config{}).Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigNormalized_FillsDefaults(t *testing.T) {
	c := kubernetesx.Config{}.Normalized()
	if c.FieldManager != kubernetesx.DefaultFieldManager {
		t.Fatalf("FieldManager default = %q, want %q", c.FieldManager, kubernetesx.DefaultFieldManager)
	}
	if c.UserAgent != kubernetesx.DefaultUserAgent {
		t.Fatalf("UserAgent default = %q, want %q", c.UserAgent, kubernetesx.DefaultUserAgent)
	}
	if c.Logger == nil {
		t.Fatalf("Logger should default to logx.DefaultLogger()")
	}
}

// -----------------------------------------------------------------------------
// Credential
// -----------------------------------------------------------------------------

const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: dev
  cluster:
    server: https://10.0.0.1:6443
    certificate-authority-data: SGVsbG8=
contexts:
- name: dev
  context:
    cluster: dev
    user: dev
current-context: dev
users:
- name: dev
  user:
    token: abcdef
`

const execKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: dev
  cluster:
    server: https://10.0.0.1:6443
contexts:
- name: dev
  context:
    cluster: dev
    user: dev
current-context: dev
users:
- name: dev
  user:
    exec:
      command: aws
      apiVersion: client.authentication.k8s.io/v1
`

func TestCredentialValidate_KubeconfigAcceptsSelfContained(t *testing.T) {
	c := kubernetesx.Credential{Kind: kubernetesx.CredentialKindKubeconfig, Kubeconfig: []byte(validKubeconfig)}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestCredentialValidate_KubeconfigRejectsExecPlugin(t *testing.T) {
	c := kubernetesx.Credential{Kind: kubernetesx.CredentialKindKubeconfig, Kubeconfig: []byte(execKubeconfig)}
	err := c.Validate()
	if err == nil {
		t.Fatalf("expected exec plugin to be rejected")
	}
	if !errors.Is(err, kubernetesx.ErrCredentialInvalid) && err.Error() == "" {
		t.Fatalf("expected non-empty error, got %v", err)
	}
}

func TestCredentialValidate_KubeconfigRejectsExternalCertFiles(t *testing.T) {
	cfg := `apiVersion: v1
kind: Config
clusters:
- name: dev
  cluster:
    server: https://10.0.0.1:6443
contexts:
- name: dev
  context: {cluster: dev, user: dev}
current-context: dev
users:
- name: dev
  user:
    client-certificate: /etc/certs/client.crt
    client-key: /etc/certs/client.key
`
	c := kubernetesx.Credential{Kind: kubernetesx.CredentialKindKubeconfig, Kubeconfig: []byte(cfg)}
	if err := c.Validate(); err == nil {
		t.Fatalf("expected external cert file reference to be rejected")
	}
}

func TestCredentialValidate_ServiceAccountRequiresHostAndToken(t *testing.T) {
	if err := (kubernetesx.Credential{Kind: kubernetesx.CredentialKindServiceAccount}).Validate(); err == nil {
		t.Fatalf("expected error for missing host/token")
	}
	c := kubernetesx.Credential{Kind: kubernetesx.CredentialKindServiceAccount, Host: "https://10.0.0.1:6443", Token: "tok"}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestCredentialValidate_InClusterIsAlwaysValid(t *testing.T) {
	if err := (kubernetesx.Credential{Kind: kubernetesx.CredentialKindInCluster}).Validate(); err != nil {
		t.Fatalf("in-cluster must be valid without fields: %v", err)
	}
}

func TestCredentialValidate_UnknownKindRejected(t *testing.T) {
	err := (kubernetesx.Credential{Kind: "WEIRD"}).Validate()
	if !errors.Is(err, kubernetesx.ErrCredentialInvalid) {
		t.Fatalf("expected ErrCredentialInvalid, got %v", err)
	}
}

func TestConfigMergeCredential_KubeconfigOverridesHost(t *testing.T) {
	base := kubernetesx.Config{Host: "https://old:6443"}
	out, err := base.MergeCredential(kubernetesx.Credential{
		Kind:       kubernetesx.CredentialKindKubeconfig,
		Kubeconfig: []byte(validKubeconfig),
		Context:    "dev",
	})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if len(out.Kubeconfig) == 0 || out.Context != "dev" {
		t.Fatalf("merge did not apply kubeconfig/context: %+v", out)
	}
	if out.Host != "https://old:6443" {
		// Host is preserved when kubeconfig is supplied; the factory reads
		// Host from the kubeconfig cluster, not from Config.Host.
		t.Fatalf("Host should be left untouched on kubeconfig merge, got %q", out.Host)
	}
}

func TestConfigMergeCredential_InClusterClearsHostAndKubeconfig(t *testing.T) {
	base := kubernetesx.Config{Host: "https://old:6443", Kubeconfig: []byte("payload")}
	out, err := base.MergeCredential(kubernetesx.Credential{Kind: kubernetesx.CredentialKindInCluster})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if out.Host != "" || len(out.Kubeconfig) != 0 {
		t.Fatalf("in-cluster merge must clear Host/Kubeconfig: %+v", out)
	}
}

// TestConfigMergeCredential_ServiceAccountCarriesTokenAndCA guards the
// regression where the SA branch only copied Host onto Config, leaving the
// factory to build an unauthenticated *rest.Config (Token/CA dropped). The
// factory reads these from Config, so MergeCredential must populate them.
func TestConfigMergeCredential_ServiceAccountCarriesTokenAndCA(t *testing.T) {
	base := kubernetesx.Config{Host: "https://old:6443", Kubeconfig: []byte("payload")}
	ca := []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----")
	out, err := base.MergeCredential(kubernetesx.Credential{
		Kind:   kubernetesx.CredentialKindServiceAccount,
		Host:   "https://10.0.0.1:6443",
		Token:  "sa-token",
		CACert: ca,
	})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if out.Host != "https://10.0.0.1:6443" {
		t.Fatalf("Host not applied: %q", out.Host)
	}
	if out.Token != "sa-token" {
		t.Fatalf("Token not carried onto Config (factory would build unauthenticated client): %q", out.Token)
	}
	if string(out.CACert) != string(ca) {
		t.Fatalf("CACert not carried onto Config: %q", out.CACert)
	}
	if len(out.Kubeconfig) != 0 || out.Context != "" {
		t.Fatalf("SA merge must clear Kubeconfig/Context: %+v", out)
	}
}

// -----------------------------------------------------------------------------
// Scheme
// -----------------------------------------------------------------------------

func TestDefaultScheme_KnowsCoreTypes(t *testing.T) {
	s := kubernetesx.DefaultScheme()
	for _, gvk := range []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Namespace"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "batch", Version: "v1", Kind: "Job"},
		{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"},
		{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
	} {
		if !s.Recognizes(gvk) {
			t.Fatalf("default scheme does not recognize %s", gvk)
		}
	}
}

// -----------------------------------------------------------------------------
// NormalizeError
// -----------------------------------------------------------------------------

func TestNormalizeError_PassesThroughErrorx(t *testing.T) {
	orig := errorx.New(errorx.CodeNotFound)
	if got := kubernetesx.NormalizeError(orig); got != orig {
		t.Fatalf("errorx errors must pass through unchanged")
	}
}

func TestNormalizeError_NilIsNil(t *testing.T) {
	if got := kubernetesx.NormalizeError(nil); got != nil {
		t.Fatalf("nil must return nil")
	}
}

func TestNormalizeError_StatusErrorBranches(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want errorx.Code
	}{
		{"not found", notFound(), kubernetesx.CodeNotFound},
		{"already exists", alreadyExists(), kubernetesx.CodeAlreadyExists},
		{"conflict", conflict(), kubernetesx.CodeConflict},
		{"unauthorized", apierrors.NewUnauthorized("no creds"), kubernetesx.CodeUnauthorized},
		{"forbidden", apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "namespaces"}, "demo", errors.New("denied")), kubernetesx.CodeForbidden},
		{"timeout", apierrors.NewTimeoutError("wait", 1), kubernetesx.CodeTimeout},
		{"server timeout", apierrors.NewServerTimeout(schema.GroupResource{Group: "", Resource: "pods"}, "list", 1), kubernetesx.CodeTimeout},
		{"service unavailable", apierrors.NewServiceUnavailable("down"), kubernetesx.CodeAPIUnavailable},
		{"internal", apierrors.NewInternalError(errors.New("boom")), kubernetesx.CodeAPIUnavailable},
		{"too many requests", apierrors.NewTooManyRequests("slow", 1), errorx.Code("KUBERNETES_TOO_MANY_REQUESTS")},
		{"bad request", apierrors.NewBadRequest("nope"), kubernetesx.CodeConfigInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kubernetesx.NormalizeError(tc.err)
			if got == nil {
				t.Fatalf("NormalizeError returned nil for %v", tc.err)
			}
			if code := errorx.CodeOf(got); code != tc.want {
				t.Fatalf("code = %s, want %s (err=%v)", code, tc.want, got)
			}
		})
	}
}

func TestNormalizeError_FieldConflictDetected(t *testing.T) {
	// A 409 with reason ApplyConflict must map to KUBERNETES_FIELD_CONFLICT,
	// not the generic KUBERNETES_CONFLICT.
	err := fieldConflictError()
	got := kubernetesx.NormalizeError(err)
	if code := errorx.CodeOf(got); code != kubernetesx.CodeFieldConflict {
		t.Fatalf("code = %s, want %s", code, kubernetesx.CodeFieldConflict)
	}
	// The cause is the apierrors.StatusError, not the ErrFieldConflict
	// sentinel, so we assert on the code rather than errors.Is.
}

func TestNormalizeError_ContextCanceledAndDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := errorx.CodeOf(kubernetesx.NormalizeError(ctx.Err())); code != kubernetesx.CodeTimeout {
		t.Fatalf("canceled -> %s, want %s", code, kubernetesx.CodeTimeout)
	}
	dctx, dcancel := context.WithTimeout(context.Background(), 0)
	defer dcancel()
	<-dctx.Done()
	if code := errorx.CodeOf(kubernetesx.NormalizeError(dctx.Err())); code != kubernetesx.CodeTimeout {
		t.Fatalf("deadline -> %s, want %s", code, kubernetesx.CodeTimeout)
	}
}

func TestNormalizeError_UnreachableDetected(t *testing.T) {
	for _, err := range []error{
		errors.New("dial tcp 10.0.0.1:6443: connect: connection refused"),
		errors.New("lookup xyz: no such host"),
		errors.New("tls: failed to verify certificate"),
		errors.New("x509: certificate signed by unknown authority"),
	} {
		got := kubernetesx.NormalizeError(err)
		if code := errorx.CodeOf(got); code != kubernetesx.CodeUnreachable {
			t.Fatalf("for %v: code = %s, want %s", err, code, kubernetesx.CodeUnreachable)
		}
	}
}

func TestNormalizeError_MetadataHasNoSecrets(t *testing.T) {
	got := kubernetesx.NormalizeError(notFound())
	// The metadata must carry api_group/kind/name but never token/key.
	meta := errorx.MetadataOf(got)
	if _, ok := meta["token"]; ok {
		t.Fatalf("metadata must not carry token: %#v", meta)
	}
	if _, ok := meta["kubeconfig"]; ok {
		t.Fatalf("metadata must not carry kubeconfig: %#v", meta)
	}
}

// -----------------------------------------------------------------------------
// Fake Client round-trip
// -----------------------------------------------------------------------------

func TestFakeClient_CRUDRoundTrip(t *testing.T) {
	c := fake.NewClient()
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("create: %v", err)
	}
	got := &corev1.Namespace{}
	if err := c.Get(ctx, ctrlclient.ObjectKey{Name: "demo"}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "demo" {
		t.Fatalf("get returned wrong name: %q", got.Name)
	}

	got.Labels = map[string]string{"env": "test"}
	if err := c.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	list := &corev1.NamespaceList{}
	if err := c.List(ctx, list); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("list len = %d, want 1", len(list.Items))
	}

	if err := c.Delete(ctx, got); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.Get(ctx, ctrlclient.ObjectKey{Name: "demo"}, &corev1.Namespace{}); errorx.CodeOf(err) != kubernetesx.CodeNotFound {
		t.Fatalf("after delete, get should be CodeNotFound, got %v", err)
	}
}

func TestFakeClient_ApplyCreatesThenIdempotent(t *testing.T) {
	c := fake.NewClient()
	ctx := context.Background()

	nsGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "applied"}}
	// The fake client's SSA path routes through unstructured, which requires
	// the object to carry its GVK (a real API server fills this in on
	// accept). The first Apply also re-serializes the object, clearing the
	// GVK, so we reset it before the idempotent re-apply.
	ns.SetGroupVersionKind(nsGVK)
	if err := c.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-namespace"}); err != nil {
		t.Fatalf("apply create: %v", err)
	}
	ns.SetGroupVersionKind(nsGVK)
	if err := c.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-namespace"}); err != nil {
		t.Fatalf("apply idempotent: %v", err)
	}
	got := &corev1.Namespace{}
	if err := c.Get(ctx, ctrlclient.ObjectKey{Name: "applied"}, got); err != nil {
		t.Fatalf("get after apply: %v", err)
	}
	if got.Name != "applied" {
		t.Fatalf("applied object name = %q", got.Name)
	}
}

func TestFakeClient_ProbeStub(t *testing.T) {
	want := kubernetesx.ProbeResult{Reachable: true, ServerVersion: kubernetesx.VersionInfo{Major: "1", Minor: "31", GitVersion: "v1.31.0"}}
	c := fake.NewClient(fake.WithProbeResult(want))
	got, err := c.Probe(context.Background(), kubernetesx.ProbeRequest{})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !got.Reachable || got.ServerVersion.GitVersion != "v1.31.0" {
		t.Fatalf("probe stub returned wrong result: %+v", got)
	}
}

func TestFakeClient_ProbeError(t *testing.T) {
	c := fake.NewClient(fake.WithProbeError(errors.New("boom")))
	if _, err := c.Probe(context.Background(), kubernetesx.ProbeRequest{}); err == nil {
		t.Fatalf("expected probe error")
	}
}

func TestFakeClient_ServerVersionStub(t *testing.T) {
	want := kubernetesx.VersionInfo{GitVersion: "v1.31.0"}
	c := fake.NewClient(fake.WithServerVersion(want))
	got, err := c.ServerVersion(context.Background())
	if err != nil {
		t.Fatalf("server version: %v", err)
	}
	if got.GitVersion != "v1.31.0" {
		t.Fatalf("got %q", got.GitVersion)
	}
}

func TestFakeClient_DiscoverStub(t *testing.T) {
	caps := kubernetesx.Capabilities{
		ServerVersion: kubernetesx.VersionInfo{GitVersion: "v1.31.0"},
		APIs: []kubernetesx.APIResource{
			{Group: "", Version: "v1", Kind: "Namespace", Namespaced: false, Verbs: []string{"create", "list"}},
		},
	}
	c := fake.NewClient(fake.WithDiscoverResult(caps))
	got, err := c.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(got.APIs) != 1 || got.APIs[0].Kind != "Namespace" {
		t.Fatalf("discover stub returned wrong apis: %+v", got.APIs)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func notFound() error {
	return apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, "demo")
}

func alreadyExists() error {
	return apierrors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "namespaces"}, "demo")
}

func conflict() error {
	return apierrors.NewConflict(schema.GroupResource{Group: "", Resource: "namespaces"}, "demo", errors.New("version mismatch"))
}

func fieldConflictError() error {
	// Build a StatusError whose Reason is "ApplyConflict" so that
	// isFieldConflict detects it.
	status := metav1.Status{
		Status:  metav1.StatusFailure,
		Message: "namespace demo is managed by field manager aisphere-hub-workload",
		Reason:  "ApplyConflict",
		Code:    409,
		Details: &metav1.StatusDetails{
			Group: "",
			Kind:  "Namespace",
			Name:  "demo",
		},
	}
	return &apierrors.StatusError{ErrStatus: status}
}
