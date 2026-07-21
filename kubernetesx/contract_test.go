package kubernetesx_test

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clientgotesting "k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/kubernetesx"
	k8sfake "github.com/aisphereio/kernel/kubernetesx/fake"
)

// contract_test.go asserts the public Client interface contract that any
// kubernetesx.Client implementation (production or fake) must satisfy. These
// are the invariants Hub business code relies on:
//
//   - Apply is idempotent for the same field manager.
//   - Apply field-ownership conflicts surface as KUBERNETES_FIELD_CONFLICT.
//   - NotFound from Get/List normalizes to KUBERNETES_NOT_FOUND.
//   - AlreadyExists from Create normalizes to KUBERNETES_ALREADY_EXISTS.
//   - Conflict from Update normalizes to KUBERNETES_CONFLICT.
//   - Escape-hatch accessors (Scheme/RESTConfig/Dynamic/Discovery) return the
//     configured values without contacting a server.
//   - NormalizeError never leaks kubeconfig/token/private-key material.

// -----------------------------------------------------------------------------
// Apply contract
// -----------------------------------------------------------------------------

func TestContract_ApplyIsIdempotent(t *testing.T) {
	c := k8sfake.NewClient()
	ctx := context.Background()
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ct-idempotent"}}
	ns.SetGroupVersionKind(gvk)
	if err := c.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-test"}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	ns.SetGroupVersionKind(gvk)
	if err := c.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-test"}); err != nil {
		t.Fatalf("idempotent re-apply must not error: %v", err)
	}
}

func TestContract_ApplyFieldConflictNormalized(t *testing.T) {
	// Construct a StatusError that mimics an SSA field-ownership conflict and
	// assert the classifier routes it to KUBERNETES_FIELD_CONFLICT, not the
	// generic conflict code. The fake client does not reproduce field-conflict
	// semantics faithfully (see package doc), so this contract is exercised
	// through NormalizeError on a representative error.
	conflict := fieldConflictError()
	got := kubernetesx.NormalizeError(conflict)
	if code := errorx.CodeOf(got); code != kubernetesx.CodeFieldConflict {
		t.Fatalf("field conflict normalized to %s, want %s", code, kubernetesx.CodeFieldConflict)
	}
	if errorx.HTTPStatusOf(got) != errorx.HTTPStatusConflict {
		t.Fatalf("field conflict http status = %d, want %d", errorx.HTTPStatusOf(got), errorx.HTTPStatusConflict)
	}
	if errorx.RetryableOf(got) {
		t.Fatalf("field conflict must not be retryable")
	}
}

// -----------------------------------------------------------------------------
// CRUD error normalization contract
// -----------------------------------------------------------------------------

func TestContract_NotFoundFromGet(t *testing.T) {
	c := k8sfake.NewClient()
	err := c.Get(context.Background(), ctrlclient.ObjectKey{Name: "missing"}, &corev1.Namespace{})
	if code := errorx.CodeOf(err); code != kubernetesx.CodeNotFound {
		t.Fatalf("get missing -> %s, want %s", code, kubernetesx.CodeNotFound)
	}
	if errorx.HTTPStatusOf(err) != errorx.HTTPStatusNotFound {
		t.Fatalf("http status = %d, want %d", errorx.HTTPStatusOf(err), errorx.HTTPStatusNotFound)
	}
}

func TestContract_AlreadyExistsFromCreate(t *testing.T) {
	c := k8sfake.NewClient(k8sfake.WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dup"}}))
	ctx := context.Background()
	err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dup"}})
	if code := errorx.CodeOf(err); code != kubernetesx.CodeAlreadyExists {
		t.Fatalf("create duplicate -> %s, want %s", code, kubernetesx.CodeAlreadyExists)
	}
}

func TestContract_ConflictFromUpdate(t *testing.T) {
	// The fake client does not reproduce optimistic-concurrency conflicts
	// faithfully (its Update does not enforce ResourceVersion checks the way
	// a real API server does). We therefore assert the conflict-normalization
	// contract directly through NormalizeError on a representative
	// apierrors.NewConflict value. Real end-to-end conflict semantics are
	// covered by envtest in a follow-up PR.
	err := kubernetesx.NormalizeError(apierrors.NewConflict(
		schema.GroupResource{Group: "", Resource: "namespaces"}, "demo", errors.New("version mismatch")))
	if code := errorx.CodeOf(err); code != kubernetesx.CodeConflict {
		t.Fatalf("update conflict -> %s, want %s", code, kubernetesx.CodeConflict)
	}
	if errorx.HTTPStatusOf(err) != errorx.HTTPStatusConflict {
		t.Fatalf("http status = %d, want %d", errorx.HTTPStatusOf(err), errorx.HTTPStatusConflict)
	}
	if !errorx.RetryableOf(err) {
		t.Fatalf("generic conflict should be retryable")
	}
}

// -----------------------------------------------------------------------------
// Escape-hatch accessor contract
// -----------------------------------------------------------------------------

func TestContract_EscapeHatchesReturnConfiguredValues(t *testing.T) {
	scheme := kubernetesx.DefaultScheme()
	c := k8sfake.NewClient(k8sfake.WithScheme(scheme))
	if c.Scheme() != scheme {
		t.Fatalf("Scheme() did not return the configured scheme")
	}
	// RESTConfig/Dynamic/Discovery default to nil on the fake client; the
	// contract is that the accessors return exactly what was configured.
	if c.RESTConfig() != nil {
		t.Fatalf("RESTConfig() should be nil when not configured")
	}
	if c.Dynamic() != nil {
		t.Fatalf("Dynamic() should be nil when not configured")
	}
	if c.Discovery() != nil {
		t.Fatalf("Discovery() should be nil when not configured")
	}
}

func TestContract_DiscoveryEscapeHatchWired(t *testing.T) {
	// When a real fake discovery client is injected, Discover must use it
	// rather than the canned Capabilities.
	fd := &fakediscovery.FakeDiscovery{Fake: &clientgotesting.Fake{}}
	fd.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{{Kind: "Namespace", Namespaced: false, Verbs: []string{"create", "list"}}},
		},
	}
	c := k8sfake.NewClient(k8sfake.WithDiscoveryClient(fd))
	caps, err := c.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	found := false
	for _, a := range caps.APIs {
		if a.Kind == "Namespace" && a.Version == "v1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("injected discovery did not surface Namespace: %+v", caps.APIs)
	}
}

// -----------------------------------------------------------------------------
// Error metadata secrecy contract
// -----------------------------------------------------------------------------

func TestContract_ErrorMetadataNeverCarriesSecrets(t *testing.T) {
	// A StatusError with a name but no secret material should produce
	// metadata containing api_group/kind/name but never token/kubeconfig.
	err := apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, "demo")
	got := kubernetesx.NormalizeError(err)
	meta := errorx.MetadataOf(got)
	for _, key := range []string{"token", "kubeconfig", "client_key", "private_key", "password"} {
		if v, ok := meta[key]; ok {
			t.Fatalf("metadata must not carry %q (got %v)", key, v)
		}
	}
	if _, ok := meta["kind"]; !ok {
		t.Fatalf("metadata should carry kind: %#v", meta)
	}
}

// -----------------------------------------------------------------------------
// Discovery flattening contract
// -----------------------------------------------------------------------------

func TestContract_FlattenResourcesSkipsVerblessAndPreservesVerbs(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Kind: "Deployment", Namespaced: true, Verbs: []string{"create", "list"}},
				{Kind: "Deployment/status", Verbs: nil}, // verbless subresource, skipped
			},
		},
	}
	out := kubernetesx.FlattenResources(lists)
	if len(out) != 1 {
		t.Fatalf("flatten len = %d, want 1 (verbless skipped)", len(out))
	}
	if out[0].Group != "apps" || out[0].Version != "v1" || out[0].Kind != "Deployment" {
		t.Fatalf("flatten produced wrong entry: %+v", out[0])
	}
	if len(out[0].Verbs) != 2 {
		t.Fatalf("verbs not preserved: %+v", out[0].Verbs)
	}
}

// -----------------------------------------------------------------------------
// ApplyOptions.ToPatchOptions contract
// -----------------------------------------------------------------------------

func TestContract_ApplyOptionsToPatchOptions(t *testing.T) {
	// Default field manager fallback.
	got := kubernetesx.ApplyOptions{}.ToPatchOptions("")
	if len(got) != 1 {
		t.Fatalf("default opts len = %d, want 1", len(got))
	}
	// Explicit field manager + force + dryrun.
	got = kubernetesx.ApplyOptions{FieldManager: "fm", ForceOwnership: true, DryRun: true}.ToPatchOptions("ignored")
	if len(got) != 3 {
		t.Fatalf("full opts len = %d, want 3", len(got))
	}
}

// ensure errors import stays referenced for helper-style assertions.
var _ = errors.Is
