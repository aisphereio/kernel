//go:build integration

// integration_test.go verifies kubernetesx.Client against a real Kubernetes
// API server. It is excluded from the default `go test` run by the build tag
// above because it needs a reachable cluster and real credentials.
//
// Run it manually:
//
//	AISPHERE_TEST_KUBECONFIG=/tmp/aisphere-test-kubeconfig.yaml \
//	AISPHERE_TEST_SERVER_NAME=apiserver.cluster.local \
//	go test ./kubernetesx/ -tags=integration -run TestIntegration -v -count=1
//
// The kubeconfig file is read from the environment and never committed; the
// test redacts all credential material from its logs.
package kubernetesx_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/kubernetesx"
)

func TestIntegration_ProbeAndApply(t *testing.T) {
	kubeconfigPath := os.Getenv("AISPHERE_TEST_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("AISPHERE_TEST_KUBECONFIG not set; skipping integration test")
	}
	raw, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("read kubeconfig: %v", err)
	}
	serverName := os.Getenv("AISPHERE_TEST_SERVER_NAME") // e.g. apiserver.cluster.local

	// Validate the credential the same way Hub would before persisting it.
	cred := kubernetesx.Credential{
		Kind:       kubernetesx.CredentialKindKubeconfig,
		Kubeconfig: raw,
	}
	if err := cred.Validate(); err != nil {
		t.Fatalf("credential rejected by Validate: %v", err)
	}

	cfg := kubernetesx.Config{
		FieldManager:  "aisphere-hub",
		Timeout:       30 * time.Second,
		QPS:           50,
		Burst:         100,
		ServerName:    serverName, // preserve TLS verification against the in-cluster DNS SAN
		Logger:        nil,        // defaults to logx.DefaultLogger()
		Metrics:       nil,        // nil-safe no-op
	}
	cfg, err = cfg.MergeCredential(cred)
	if err != nil {
		t.Fatalf("merge credential: %v", err)
	}

	client, err := kubernetesx.New(cfg)
	if err != nil {
		t.Fatalf("kubernetesx.New: %v", err)
	}

	ctx := context.Background()

	// --- Probe ---
	t.Run("Probe", func(t *testing.T) {
		result, err := client.Probe(ctx, kubernetesx.ProbeRequest{})
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		// Redact: never log credentials. ProbeResult carries no secrets.
		t.Logf("probe: reachable=%v server=%s uid=%s latency=%s readonly=%v warnings=%v access=%+v apis=%d",
			result.Reachable, result.ServerVersion.GitVersion, result.ClusterUID,
			result.Latency, result.NamespaceAccess.ReadOnly, result.Warnings, result.NamespaceAccess, len(result.APIs))
		if !result.Reachable {
			t.Fatalf("cluster not reachable")
		}
		if result.ServerVersion.GitVersion == "" {
			t.Fatalf("no server version returned")
		}
		if result.ClusterUID == "" {
			t.Fatalf("no cluster uid (kube-system namespace) returned")
		}
		if !result.NamespaceAccess.CanView {
			t.Fatalf("credential cannot list namespaces; access review failed")
		}
	})

	// --- ServerVersion / Discover ---
	t.Run("ServerVersion", func(t *testing.T) {
		v, err := client.ServerVersion(ctx)
		if err != nil {
			t.Fatalf("server version: %v", err)
		}
		t.Logf("server version: %s (major=%s minor=%s platform=%s)", v.GitVersion, v.Major, v.Minor, v.Platform)
	})
	t.Run("Discover", func(t *testing.T) {
		caps, err := client.Discover(ctx)
		if err != nil {
			t.Fatalf("discover: %v", err)
		}
		// Sanity: a Kubernetes cluster always has core/v1 Namespace.
		if !caps.HasAPI("", "v1", "Namespace") {
			t.Fatalf("discovery did not surface core/v1 Namespace: %d apis", len(caps.APIs))
		}
		t.Logf("discovery: %d apis, server=%s", len(caps.APIs), caps.ServerVersion.GitVersion)
	})

	// --- Apply (SSA) a test namespace, then clean up ---
	nsName := "kubernetesx-it-" + stableSuffix()
	nsGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	t.Run("ApplyNamespace", func(t *testing.T) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Labels: map[string]string{
					"aisphere.io/managed-by":    "aisphere-hub",
					"aisphere.io/integration-test": "true",
				},
			},
		}
		ns.SetGroupVersionKind(nsGVK)
		if err := client.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-namespace"}); err != nil {
			t.Fatalf("apply namespace: %v", err)
		}
		// Idempotent re-apply.
		ns.SetGroupVersionKind(nsGVK)
		if err := client.Apply(ctx, ns, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-namespace"}); err != nil {
			t.Fatalf("idempotent re-apply: %v", err)
		}
		got := &corev1.Namespace{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Name: nsName}, got); err != nil {
			t.Fatalf("get applied namespace: %v", err)
		}
		if got.Name != nsName {
			t.Fatalf("applied namespace name = %q, want %q", got.Name, nsName)
		}
		if got.Labels["aisphere.io/managed-by"] != "aisphere-hub" {
			t.Fatalf("SSA did not set managed-by label: %+v", got.Labels)
		}
		t.Logf("applied namespace %s (uid=%s, resourceVersion=%s)", got.Name, got.UID, got.ResourceVersion)
	})

	// --- Error normalization against the real API server ---
	t.Run("GetMissingNormalizesToNotFound", func(t *testing.T) {
		err := client.Get(ctx, ctrlclient.ObjectKey{Name: "kubernetesx-definitely-missing"}, &corev1.Namespace{})
		if err == nil {
			t.Fatalf("expected error for missing namespace")
		}
		if code := errorx.CodeOf(err); code != kubernetesx.CodeNotFound {
			t.Fatalf("missing namespace -> code %s, want %s (err=%v)", code, kubernetesx.CodeNotFound, err)
		}
		t.Logf("not-found normalized: code=%s http=%d", errorx.CodeOf(err), errorx.HTTPStatusOf(err))
	})

	// --- Cleanup ---
	t.Run("Cleanup", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
		ns.SetGroupVersionKind(nsGVK)
		if err := client.Delete(ctx, ns); err != nil {
			t.Logf("cleanup delete %s: %v (leaving for manual inspection)", nsName, err)
		} else {
			t.Logf("cleaned up namespace %s", nsName)
		}
	})
}

// stableSuffix returns a short, stable suffix for the test namespace so
// re-runs against the same cluster reuse the same namespace name and the
// SSA apply is genuinely idempotent. We derive it from the server name env
// rather than a random value so failures are reproducible.
func stableSuffix() string {
	host := os.Getenv("AISPHERE_TEST_SERVER_NAME")
	if host == "" {
		host = "default"
	}
	// Keep it DNS-safe and short.
	if len(host) > 16 {
		host = host[:16]
	}
	// Replace dots with dashes for DNS label safety.
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			host = host[:i] + "-" + host[i+1:]
		}
	}
	return host
}

var _ = fmt.Sprintf
