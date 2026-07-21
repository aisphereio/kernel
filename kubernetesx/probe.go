package kubernetesx

import (
	"context"
	"time"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ProbeRequest controls what Probe validates. The zero value requests the full
// probe: reachability, server version, cluster UID, API list, and namespace
// access review. Fields can narrow the work for periodic re-probes.
type ProbeRequest struct {
	// SkipAccessReview, when true, skips the create/update/delete
	// SelfSubjectAccessReview calls. Use this for cheap reachability checks
	// where the access matrix is already known.
	SkipAccessReview bool

	// Namespace is the namespace used as the subject of the access review.
	// Empty means a cluster-scoped review (namespace create/delete are
	// cluster-scoped; update is reviewed against this namespace when set).
	Namespace string
}

// AccessReview summarizes the credential's namespace permissions. It tells Hub
// whether a cluster is writable (can onboard namespaces) or read-only.
type AccessReview struct {
	CanView   bool `json:"can_view"`
	CanCreate bool `json:"can_create"`
	CanUpdate bool `json:"can_update"`
	CanDelete bool `json:"can_delete"`
	// ReadOnly is true when CanCreate/CanUpdate/CanDelete are all false but
	// CanView is true. Hub labels such clusters as read-only.
	ReadOnly bool `json:"read_only"`
}

// ProbeResult is the full output of a probe. Latency covers the whole probe
// round-trip, not individual sub-calls.
type ProbeResult struct {
	Reachable       bool         `json:"reachable"`
	ServerVersion   VersionInfo  `json:"server_version"`
	ClusterUID      string       `json:"cluster_uid"`
	APIs            []APIResource `json:"apis"`
	NamespaceAccess AccessReview  `json:"namespace_access"`
	Latency         time.Duration `json:"latency_ns"`
	Warnings        []string     `json:"warnings"`
}

// probe runs the full probe against the supplied raw client and discovery
// client. It is shared by Client.Probe.
func probe(ctx context.Context, raw ctrlclient.Client, d discoveryClient, req ProbeRequest, scheme *runtime.Scheme) (ProbeResult, error) {
	start := time.Now()
	result := ProbeResult{}

	caps, derr := discover(ctx, d)
	if derr != nil {
		// Discovery failure most often means unreachable / unauthorized.
		return result, derr
	}
	result.Reachable = true
	result.ServerVersion = caps.ServerVersion
	result.APIs = caps.APIs

	// Cluster UID: kube-system namespace UID is a stable cluster identifier
	// widely used by operators. We read it once; missing kube-system is not
	// fatal but is recorded as a warning.
	uid, warn := clusterUID(ctx, raw)
	result.ClusterUID = uid
	if warn != "" {
		result.Warnings = append(result.Warnings, warn)
	}

	// Namespace view: list namespaces. A forbidden response downgrades
	// CanView but does not fail the probe — a cluster may legitimately
	// restrict namespace listing while still allowing create.
	result.NamespaceAccess.CanView = canListNamespaces(ctx, raw, req.Namespace)
	if !result.NamespaceAccess.CanView {
		result.Warnings = append(result.Warnings, "credential cannot list namespaces")
	}

	if !req.SkipAccessReview {
		result.NamespaceAccess.CanCreate = canDo(ctx, raw, "create", "namespaces", "")
		result.NamespaceAccess.CanUpdate = canDo(ctx, raw, "update", "namespaces", req.Namespace)
		result.NamespaceAccess.CanDelete = canDo(ctx, raw, "delete", "namespaces", "")
	}
	result.NamespaceAccess.ReadOnly = result.NamespaceAccess.CanView &&
		!result.NamespaceAccess.CanCreate &&
		!result.NamespaceAccess.CanUpdate &&
		!result.NamespaceAccess.CanDelete

	result.Latency = time.Since(start)
	return result, nil
}

// clusterUID returns the UID of the kube-system namespace. It is the canonical
// cross-component cluster identifier.
func clusterUID(ctx context.Context, raw ctrlclient.Client) (string, string) {
	ns := &corev1.Namespace{}
	if err := raw.Get(ctx, ctrlclient.ObjectKey{Name: "kube-system"}, ns); err != nil {
		return "", "failed to read kube-system namespace uid: " + err.Error()
	}
	return string(ns.UID), ""
}

// canListNamespaces reports whether the credential can list namespaces. We
// attempt an actual List limited to a single item; a forbidden/forbidden-style
// error means no view permission. This avoids a SelfSubjectAccessReview round
// trip for the common case.
func canListNamespaces(ctx context.Context, raw ctrlclient.Client, namespace string) bool {
	list := &corev1.NamespaceList{}
	opts := []ctrlclient.ListOption{ctrlclient.Limit(1)}
	if namespace != "" {
		opts = append(opts, ctrlclient.InNamespace(namespace))
	}
	if err := raw.List(ctx, list, opts...); err != nil {
		return false
	}
	return true
}

// canDo submits a SelfSubjectAccessReview for the given verb/resource and
// returns whether the API server reports Allowed=true. Errors default to
// false so that a broken review never grants capability.
func canDo(ctx context.Context, raw ctrlclient.Client, verb, resource, namespace string) bool {
	ssar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:      verb,
				Resource:  resource,
				Group:     "",
				Namespace: namespace,
			},
		},
	}
	if err := raw.Create(ctx, ssar, &ctrlclient.CreateOptions{}); err != nil {
		return false
	}
	return ssar.Status.Allowed
}

// ensure imports are anchored for future subresource-style probes.
var _ = metav1.ObjectMeta{}
