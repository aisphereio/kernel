// Package kubernetesx provides the unified Kubernetes SDK abstraction for
// Aisphere Kernel.
//
// kubernetesx is the ONLY Kubernetes client abstraction that Hub business code
// should depend on. It exposes a stable, application-oriented Client interface
// backed by controller-runtime and client-go, with Server-Side Apply, discovery,
// probe, error normalization, and metrics hooks.
//
// Kernel owns the Kubernetes SDK surface only. It does NOT persist cluster
// records, store kubeconfig, manage organizations/users/shares, or define the
// Cluster/Namespace product model — those belong to Hub. Business code must
// never call client-go / controller-runtime directly.
//
// # Design principle
//
// kubernetesx wraps the raw Kubernetes SDK into an application-layer surface so
// that Hub request handlers stay free of client-go types:
//
//	HTTP/gRPC Request
//	  -> authz (Hub)
//	  -> kubernetesx.Client
//	  -> Kubernetes API Server
//	  -> response
//
// Phase 1 is request-driven: no per-cluster Manager, Informer, Cache, or
// long-lived Watch. Periodic probe and state sync run through taskx. A later
// phase may add a dedicated controller-runtime process when WarmPool, Sandbox
// Controller, or long-lived Watch is genuinely needed.
//
// # 30-second quickstart
//
//	client, err := kubernetesx.New(kubernetesx.Config{
//	    Host:         "https://10.0.0.1:6443",
//	    Kubeconfig:   kubeconfigBytes,
//	    FieldManager: "aisphere-hub",
//	    QPS:          50,
//	    Burst:        100,
//	    Timeout:      30 * time.Second,
//	    Logger:       logx.DefaultLogger(),
//	    Metrics:      metricsx.Noop(),
//	})
//	if err != nil { return err }
//
//	// Server-Side Apply a Namespace
//	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
//	err = client.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-namespace"})
//
//	// Probe before registering a cluster
//	result, err := client.Probe(ctx, kubernetesx.ProbeRequest{})
//
// # Server-Side Apply
//
// AISphere-managed resources (Namespace, Pod, CRD, Sandbox) use SSA. Each
// resource class uses its own Field Manager (e.g. aisphere-hub-namespace) and
// only owns the fields it explicitly declares. ForceOwnership is allowed only
// for fields AISphere exclusively owns. Field conflicts surface as
// KUBERNETES_FIELD_CONFLICT and must never be silently overwritten.
//
// # Errors
//
// All foreign Kubernetes errors are normalized through NormalizeError into
// stable errorx.Code constants (KUBERNETES_*). Error metadata never carries
// kubeconfig, token, or client-certificate private key material.
//
// # Escape hatches
//
// RESTConfig(), Dynamic(), and Discovery() expose raw client-go objects. They
// are infrastructure-layer escape hatches: ONLY Hub Data Adapter code may use
// them. Hub Biz layer must not depend on these.
//
// # Forbidden patterns
//
// Do not import `k8s.io/client-go`, `k8s.io/apimachinery`, or
// `sigs.k8s.io/controller-runtime` in business code. Use the kubernetesx.Client
// interface.
package kubernetesx
