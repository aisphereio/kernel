package kubernetesx

import (
	"context"

	"github.com/aisphereio/kernel/logx"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is the application-oriented Kubernetes client used by Hub. It wraps
// controller-runtime's client with Server-Side Apply, discovery, probe, and
// stable error normalization, so that Hub request handlers never touch
// client-go types directly.
//
// The CRUD methods mirror controller-runtime's client.Client signatures so
// that typed objects and unstructured objects both work without translation.
// Apply and ApplyUnstructured add SSA on top. ServerVersion/Discover/Probe
// cover cluster onboarding and periodic health checks.
//
// RESTConfig, Dynamic, and Discovery are escape hatches: ONLY Hub Data
// Adapter code may call them. Hub Biz layer must not depend on these.
type Client interface {
	// --- CRUD (delegate to controller-runtime client) ---

	Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error
	List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error
	Create(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error
	Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error
	Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error
	Delete(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error

	// --- Server-Side Apply ---

	// Apply runs SSA on a typed object. Field ownership conflicts surface as
	// KUBERNETES_FIELD_CONFLICT via NormalizeError; they are never silently
	// overwritten.
	Apply(ctx context.Context, obj ctrlclient.Object, opts ApplyOptions) error

	// ApplyUnstructured runs SSA on an *unstructured.Unstructured, used for
	// third-party CRDs that have no generated Go type.
	ApplyUnstructured(ctx context.Context, obj ctrlclient.Object, opts ApplyOptions) error

	// --- Cluster introspection ---

	ServerVersion(ctx context.Context) (VersionInfo, error)
	Discover(ctx context.Context) (Capabilities, error)
	Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error)

	// --- Escape hatches (Hub Data Adapter only) ---

	Scheme() *runtime.Scheme
	RESTConfig() *rest.Config
	Dynamic() dynamic.Interface
	Discovery() discovery.DiscoveryInterface
}

// client is the production Client backed by controller-runtime.
type client struct {
	raw          ctrlclient.Client
	restConfig   *rest.Config
	scheme       *runtime.Scheme
	dyn          dynamic.Interface
	disco        discovery.DiscoveryInterface
	fieldManager string
	logger       logx.Logger
}

// Get retrieves obj from the API server.
func (c *client) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	err := c.raw.Get(ctx, key, obj, opts...)
	if err != nil {
		return c.attachObjectMeta(err, obj)
	}
	return nil
}

// List retrieves a list of objects from the API server.
func (c *client) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	err := c.raw.List(ctx, list, opts...)
	return NormalizeError(err)
}

// Create creates obj on the API server.
func (c *client) Create(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error {
	err := c.raw.Create(ctx, obj, opts...)
	return c.attachObjectMeta(err, obj)
}

// Update updates obj on the API server.
func (c *client) Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error {
	err := c.raw.Update(ctx, obj, opts...)
	return c.attachObjectMeta(err, obj)
}

// Patch patches obj on the API server. Use client.Apply as the patch to perform
// raw SSA without the kubernetesx.ApplyOptions wrapper.
func (c *client) Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
	err := c.raw.Patch(ctx, obj, patch, opts...)
	return c.attachObjectMeta(err, obj)
}

// Delete deletes obj from the API server.
func (c *client) Delete(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
	err := c.raw.Delete(ctx, obj, opts...)
	return c.attachObjectMeta(err, obj)
}

// Apply runs Server-Side Apply on a typed object.
func (c *client) Apply(ctx context.Context, obj ctrlclient.Object, opts ApplyOptions) error {
	err := applySSA(ctx, c.raw, obj, opts, c.fieldManager)
	if err != nil {
		return c.attachObjectMeta(err, obj)
	}
	return nil
}

// ApplyUnstructured runs Server-Side Apply on an unstructured object.
func (c *client) ApplyUnstructured(ctx context.Context, obj ctrlclient.Object, opts ApplyOptions) error {
	// controller-runtime routes unstructured objects through its
	// unstructuredClient automatically; the same applySSA path works.
	err := applySSA(ctx, c.raw, obj, opts, c.fieldManager)
	if err != nil {
		return c.attachObjectMeta(err, obj)
	}
	return nil
}

// ServerVersion returns the API server version.
func (c *client) ServerVersion(ctx context.Context) (VersionInfo, error) {
	select {
	case <-ctx.Done():
		return VersionInfo{}, NormalizeError(ctx.Err())
	default:
	}
	info, err := c.disco.ServerVersion()
	if err != nil {
		return VersionInfo{}, NormalizeError(err)
	}
	return FromVersionInfo(info), nil
}

// Discover returns the server version and the full API resource list.
func (c *client) Discover(ctx context.Context) (Capabilities, error) {
	return discover(ctx, c.disco)
}

// Probe runs the full cluster onboarding probe.
func (c *client) Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error) {
	return probe(ctx, c.raw, c.disco, req, c.scheme)
}

// Scheme returns the runtime.Scheme used by this client.
func (c *client) Scheme() *runtime.Scheme { return c.scheme }

// RESTConfig returns the underlying *rest.Config. Hub Data Adapter only.
func (c *client) RESTConfig() *rest.Config { return c.restConfig }

// Dynamic returns the dynamic client. Hub Data Adapter only.
func (c *client) Dynamic() dynamic.Interface { return c.dyn }

// Discovery returns the discovery client. Hub Data Adapter only.
func (c *client) Discovery() discovery.DiscoveryInterface { return c.disco }

// attachObjectMeta normalizes an error and stamps it with the object's
// group/kind/namespace/name for observability.
func (c *client) attachObjectMeta(err error, obj ctrlclient.Object) error {
	if err == nil {
		return nil
	}
	nerr := NormalizeError(err)
	if obj == nil {
		return nerr
	}
	group, kind := gvkFromObject(obj)
	return withObjectMetadata(nerr, group, kind, obj.GetNamespace(), obj.GetName())
}
