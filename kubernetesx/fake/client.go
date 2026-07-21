// Package fake provides a kubernetesx.Client backed by controller-runtime's
// fake client for unit tests. It is the pure-Go test surface described in
// design §4.8: no API server, no envtest, no Kind. Real SSA field-conflict
// semantics require envtest and live in a follow-up PR.
//
// Use NewClient to build a Client with pre-seeded objects and configurable
// ServerVersion / Discover / Probe stubs. CRUD and Apply delegate to the
// underlying controller-runtime fake client.
package fake

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/aisphereio/kernel/kubernetesx"
)

// Option configures the fake Client.
type Option func(*builder)

type builder struct {
	scheme      *runtime.Scheme
	objs        []ctrlclient.Object
	runtimeObjs []runtime.Object
	restConfig  *rest.Config
	dyn         dynamic.Interface
	disco       discovery.DiscoveryInterface

	// Probe / Discover stubs. When nil, the fake client returns a canned
	// reachable result so that tests do not need to set them for CRUD-only
	// scenarios.
	probeResult   kubernetesx.ProbeResult
	probeErr      error
	discoverCaps  kubernetesx.Capabilities
	discoverErr   error
	versionInfo   kubernetesx.VersionInfo
	versionErr    error
	fieldManager  string
}

// WithScheme sets the runtime.Scheme used by the fake client. Defaults to
// kubernetesx.DefaultScheme().
func WithScheme(s *runtime.Scheme) Option {
	return func(b *builder) { b.scheme = s }
}

// WithObjects seeds the fake client with typed objects.
func WithObjects(objs ...ctrlclient.Object) Option {
	return func(b *builder) { b.objs = append(b.objs, objs...) }
}

// WithRuntimeObjects seeds the fake client with runtime.Object values (useful
// for objects whose type is not a client.Object at compile time).
func WithRuntimeObjects(objs ...runtime.Object) Option {
	return func(b *builder) { b.runtimeObjs = append(b.runtimeObjs, objs...) }
}

// WithRESTConfig sets the *rest.Config returned by RESTConfig(). Tests only.
func WithRESTConfig(cfg *rest.Config) Option {
	return func(b *builder) { b.restConfig = cfg }
}

// WithDynamicClient sets the dynamic.Interface returned by Dynamic().
func WithDynamicClient(d dynamic.Interface) Option {
	return func(b *builder) { b.dyn = d }
}

// WithDiscoveryClient sets the discovery client returned by Discovery().
func WithDiscoveryClient(d discovery.DiscoveryInterface) Option {
	return func(b *builder) { b.disco = d }
}

// WithProbeResult sets the canned ProbeResult returned by Probe(). When
// WithProbeError is also set, the error wins.
func WithProbeResult(r kubernetesx.ProbeResult) Option {
	return func(b *builder) { b.probeResult = r }
}

// WithProbeError makes Probe return err instead of the canned result.
func WithProbeError(err error) Option {
	return func(b *builder) { b.probeErr = err }
}

// WithDiscoverResult sets the canned Capabilities returned by Discover().
func WithDiscoverResult(c kubernetesx.Capabilities) Option {
	return func(b *builder) { b.discoverCaps = c }
}

// WithDiscoverError makes Discover return err.
func WithDiscoverError(err error) Option {
	return func(b *builder) { b.discoverErr = err }
}

// WithServerVersion sets the canned VersionInfo returned by ServerVersion().
func WithServerVersion(v kubernetesx.VersionInfo) Option {
	return func(b *builder) { b.versionInfo = v }
}

// WithServerVersionError makes ServerVersion return err.
func WithServerVersionError(err error) Option {
	return func(b *builder) { b.versionErr = err }
}

// WithFieldManager sets the default SSA field manager used by Apply.
func WithFieldManager(fm string) Option {
	return func(b *builder) { b.fieldManager = fm }
}

// NewClient builds a kubernetesx.Client backed by controller-runtime's fake
// client. CRUD and Apply run against the in-memory fake; ServerVersion,
// Discover, and Probe return the configured stubs (or a canned reachable
// result when unset).
func NewClient(opts ...Option) kubernetesx.Client {
	b := &builder{
		fieldManager: kubernetesx.DefaultFieldManager,
	}
	for _, o := range opts {
		o(b)
	}
	scheme := b.scheme
	if scheme == nil {
		scheme = kubernetesx.DefaultScheme()
	}
	fb := fakectrl.NewClientBuilder().WithScheme(scheme)
	if len(b.objs) > 0 {
		fb = fb.WithObjects(b.objs...)
	}
	if len(b.runtimeObjs) > 0 {
		fb = fb.WithRuntimeObjects(b.runtimeObjs...)
	}

	raw := fb.Build()

	return &fakeClient{
		builder: b,
		raw:     raw,
		scheme:  scheme,
	}
}

// fakeClient implements kubernetesx.Client by delegating CRUD/Apply to the
// controller-runtime fake client and serving introspection methods from the
// configured stubs.
type fakeClient struct {
	*builder
	raw    ctrlclient.WithWatch
	scheme *runtime.Scheme
}

func (c *fakeClient) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	err := c.raw.Get(ctx, key, obj, opts...)
	return kubernetesx.NormalizeError(err)
}

func (c *fakeClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	err := c.raw.List(ctx, list, opts...)
	return kubernetesx.NormalizeError(err)
}

func (c *fakeClient) Create(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error {
	err := c.raw.Create(ctx, obj, opts...)
	return kubernetesx.NormalizeError(err)
}

func (c *fakeClient) Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error {
	err := c.raw.Update(ctx, obj, opts...)
	return kubernetesx.NormalizeError(err)
}

func (c *fakeClient) Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
	err := c.raw.Patch(ctx, obj, patch, opts...)
	return kubernetesx.NormalizeError(err)
}

func (c *fakeClient) Delete(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
	err := c.raw.Delete(ctx, obj, opts...)
	return kubernetesx.NormalizeError(err)
}

func (c *fakeClient) Apply(ctx context.Context, obj ctrlclient.Object, opts kubernetesx.ApplyOptions) error {
	// Delegate to the shared SSA helper so that Apply semantics stay
	// identical between production and fake clients. The fake client
	// implements SSA via applyPatch; field-conflict detection on the fake
	// client is limited (see package doc).
	return applyViaFake(ctx, c.raw, obj, opts, c.fieldManager)
}

func (c *fakeClient) ApplyUnstructured(ctx context.Context, obj ctrlclient.Object, opts kubernetesx.ApplyOptions) error {
	return applyViaFake(ctx, c.raw, obj, opts, c.fieldManager)
}

func (c *fakeClient) ServerVersion(ctx context.Context) (kubernetesx.VersionInfo, error) {
	if c.versionErr != nil {
		return kubernetesx.VersionInfo{}, kubernetesx.NormalizeError(c.versionErr)
	}
	return c.versionInfo, nil
}

func (c *fakeClient) Discover(ctx context.Context) (kubernetesx.Capabilities, error) {
	if c.discoverErr != nil {
		return kubernetesx.Capabilities{}, kubernetesx.NormalizeError(c.discoverErr)
	}
	if c.disco != nil {
		return discoverViaClient(ctx, c.disco)
	}
	return c.discoverCaps, nil
}

func (c *fakeClient) Probe(ctx context.Context, req kubernetesx.ProbeRequest) (kubernetesx.ProbeResult, error) {
	if c.probeErr != nil {
		return kubernetesx.ProbeResult{}, kubernetesx.NormalizeError(c.probeErr)
	}
	return c.probeResult, nil
}

func (c *fakeClient) Scheme() *runtime.Scheme            { return c.scheme }
func (c *fakeClient) RESTConfig() *rest.Config           { return c.restConfig }
func (c *fakeClient) Dynamic() dynamic.Interface         { return c.dyn }
func (c *fakeClient) Discovery() discovery.DiscoveryInterface { return c.disco }

// applyViaFake is a thin wrapper kept in this package so the fake build does
// not depend on kubernetesx.applySSA (unexported). It performs the same
// Patch+client.Apply call, with the same server-owned-field preparation as the
// production client, and routes errors through NormalizeError.
func applyViaFake(ctx context.Context, raw ctrlclient.Client, obj ctrlclient.Object, opts kubernetesx.ApplyOptions, defaultFM string) error {
	// Mirror kubernetesx.prepareForSSA: managedFields/resourceVersion must
	// be empty in an apply payload, even on the fake client, so that the
	// fake and production clients behave identically.
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	patchOpts := opts.ToPatchOptions(defaultFM)
	err := raw.Patch(ctx, obj, ctrlclient.Apply, patchOpts...)
	return kubernetesx.NormalizeError(err)
}

// discoverViaClient calls the real discovery helper when a discovery client is
// injected. This path is exercised by tests that want to feed a real
// fake discovery client rather than canned Capabilities.
func discoverViaClient(ctx context.Context, d discovery.DiscoveryInterface) (kubernetesx.Capabilities, error) {
	// We re-implement the minimal discover path here because
	// kubernetesx.discover is unexported. The logic is intentionally
	// identical.
	select {
	case <-ctx.Done():
		return kubernetesx.Capabilities{}, kubernetesx.NormalizeError(ctx.Err())
	default:
	}
	info, err := d.ServerVersion()
	if err != nil {
		return kubernetesx.Capabilities{}, kubernetesx.NormalizeError(err)
	}
	_, lists, err := d.ServerGroupsAndResources()
	if err != nil {
		return kubernetesx.Capabilities{ServerVersion: kubernetesx.FromVersionInfo(info)}, kubernetesx.NormalizeError(err)
	}
	return kubernetesx.Capabilities{
		ServerVersion: kubernetesx.FromVersionInfo(info),
		APIs:          kubernetesx.FlattenResources(lists),
	}, nil
}
