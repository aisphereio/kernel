package kubernetesx

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyOptions controls Server-Side Apply behavior. Zero-value FieldManager
// falls back to Config.FieldManager (and then DefaultFieldManager).
type ApplyOptions struct {
	// FieldManager is the SSA field owner. Each AISphere resource class
	// (namespace, sandbox, workload, network) should use its own manager so
	// that the API server tracks field ownership per concern.
	FieldManager string

	// ForceOwnership, when true, re-acquires fields owned by other managers
	// during apply. Use only for fields AISphere exclusively owns; never
	// set it to silently overwrite a collaborator (HPA, operator, …).
	ForceOwnership bool

	// DryRun, when true, validates the apply without persisting. The API
	// server still reports conflicts.
	DryRun bool
}

// toPatchOptions translates ApplyOptions into the SSA patch options that
// accompany client.Apply (the patch type itself is returned separately because
// controller-runtime's Patch signature takes Patch and PatchOption as
// distinct argument kinds).
func (o ApplyOptions) toPatchOptions(defaultFieldManager string) []ctrlclient.PatchOption {
	return o.ToPatchOptions(defaultFieldManager)
}

// ToPatchOptions is the exported form of toPatchOptions. It is exposed so that
// the fake subpackage and external test doubles can build SSA patch options
// without re-implementing the default-field-manager fallback.
func (o ApplyOptions) ToPatchOptions(defaultFieldManager string) []ctrlclient.PatchOption {
	fm := o.FieldManager
	if fm == "" {
		fm = defaultFieldManager
		if fm == "" {
			fm = DefaultFieldManager
		}
	}
	opts := []ctrlclient.PatchOption{ctrlclient.FieldOwner(fm)}
	if o.ForceOwnership {
		opts = append(opts, ctrlclient.ForceOwnership)
	}
	if o.DryRun {
		opts = append(opts, ctrlclient.DryRunAll)
	}
	return opts
}

// applySSA runs a Server-Side Apply against the supplied raw controller-runtime
// client. It is shared by Client.Apply (typed objects) and
// Client.ApplyUnstructured.
func applySSA(ctx context.Context, raw ctrlclient.Client, obj runtime.Object, opts ApplyOptions, defaultFieldManager string) error {
	patchOpts := opts.toPatchOptions(defaultFieldManager)
	// Prepare the object for SSA. Server-Side Apply requires the client NOT
	// to send managedFields or resourceVersion: managedFields is owned and
	// maintained by the API server, and resourceVersion would turn the
	// apply into an optimistic-concurrency check (defeating SSA idempotency
	// and surfacing spurious conflicts). Callers frequently reuse an
	// object that a previous Get/Apply filled with these fields, so clear
	// them here to keep Apply genuinely idempotent and call-site-friendly.
	prepareForSSA(obj.(ctrlclient.Object))
	// controller-runtime's Patch with client.Apply performs SSA: on a
	// missing object it creates, on an existing object it merges owned
	// fields and reports conflicts.
	err := raw.Patch(ctx, obj.(ctrlclient.Object), ctrlclient.Apply, patchOpts...)
	if err == nil {
		return nil
	}
	return normalizeApplyError(err)
}

// prepareForSSA clears server-owned fields that must not appear in an apply
// payload: managedFields and resourceVersion. It preserves GVK, name,
// namespace, labels, annotations, spec, and all caller-owned fields.
func prepareForSSA(obj ctrlclient.Object) {
	obj.SetManagedFields(nil)
	// resourceVersion must be empty for apply; SetResourceVersion on a
	// typed object with "" is a no-op that satisfies the contract.
	obj.SetResourceVersion("")
	// finalizers are intentionally NOT cleared: some SSA flows legitimately
	// declare finalizers they own. Only managedFields/resourceVersion are
	// universal apply blockers.
}

// normalizeApplyError surfaces SSA field-ownership conflicts as
// KUBERNETES_FIELD_CONFLICT so callers can branch without parsing messages.
// Other apierrors flow through NormalizeError.
func normalizeApplyError(err error) error {
	if err == nil {
		return nil
	}
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) && apierrors.IsConflict(statusErr) {
		if isFieldConflict(statusErr.Status()) {
			return NormalizeError(ErrFieldConflict)
		}
	}
	return NormalizeError(err)
}

// ensure unstructured is referenced for the ApplyUnstructured contract even
// when the helper above only handles runtime.Object.
var _ = unstructured.Unstructured{}
