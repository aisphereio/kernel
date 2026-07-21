package kubernetesx

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewUnstructured builds an *unstructured.Unstructured with the supplied GVK
// and namespaced name. It is the entry point for third-party CRDs that have no
// generated Go type. The returned object is ready for ApplyUnstructured.
func NewUnstructured(gvk schema.GroupVersionKind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// UnstructuredFromObject converts a typed client.Object into an
// *unstructured.Unstructured via the runtime.Scheme used by the client. It is
// useful when an apply caller has a typed object but wants to manage only a
// subset of fields through the unstructured path.
func UnstructuredFromObject(scheme *runtime.Scheme, obj runtime.Object) (*unstructured.Unstructured, error) {
	if scheme == nil {
		scheme = DefaultScheme()
	}
	out := &unstructured.Unstructured{}
	if err := scheme.Convert(obj, out, nil); err != nil {
		return nil, err
	}
	// Preserve GVK: scheme.Convert may drop it on the destination.
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		if gvks, _, err := scheme.ObjectKinds(obj); err == nil && len(gvks) > 0 {
			gvk = gvks[0]
		}
	}
	if !gvk.Empty() {
		out.SetGroupVersionKind(gvk)
	}
	return out, nil
}
