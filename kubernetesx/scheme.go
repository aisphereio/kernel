package kubernetesx

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// AddToScheme registers the default Kubernetes API groups a kubernetesx client
// understands out of the box. Callers may extend the scheme via WithScheme.
type AddToScheme = func(*runtime.Scheme) error

// defaultAddToSchemes is the set of schemes registered by DefaultScheme.
var defaultAddToSchemes = []AddToScheme{
	clientgoscheme.AddToScheme,
	v1.AddToScheme,
}

// DefaultScheme returns a fresh runtime.Scheme pre-registered with the API
// groups AISphere commonly manages:
//
//   - core/v1 (Pod, Namespace, Service, ConfigMap, Secret, …)
//   - apps/v1 (Deployment, StatefulSet, DaemonSet, ReplicaSet)
//   - batch/v1 (Job, CronJob)
//   - networking.k8s.io/v1 (Ingress, NetworkPolicy)
//   - rbac.authorization.k8s.io/v1 (Role, ClusterRole, Binding)
//   - apiextensions.k8s.io/v1 (CustomResourceDefinition)
//
// The returned scheme is a new instance; mutating it does not affect future
// callers. Third-party CRD types should be injected via WithScheme at
// construction time.
func DefaultScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	metav1AddToScheme(scheme)
	for _, add := range defaultAddToSchemes {
		if err := add(scheme); err != nil {
			// The default schemes are compile-time known-good; a failure
			// here indicates a version skew between k8s.io/* modules and
			// is not recoverable at runtime.
			panic("kubernetesx: failed to register default scheme: " + err.Error())
		}
	}
	return scheme
}

// metav1AddToScheme registers meta/v1 ListMeta / ObjectMeta onto the scheme so
// that unstructured and partial-object handling works without a separate
// registration step.
func metav1AddToScheme(scheme *runtime.Scheme) {
	// clientgoscheme.AddToScheme already registers the meta metatype; this
	// helper exists as an explicit, version-stable anchor for tests.
	scheme.AddKnownTypes(schema.GroupVersion{Group: "", Version: "v1"})
}

// mergeSchemes applies add into dst, returning the first error encountered.
// It is used by the factory to fold user-supplied WithScheme additions into
// the default scheme.
func mergeSchemes(dst *runtime.Scheme, add AddToScheme) error {
	if add == nil {
		return nil
	}
	return add(dst)
}
