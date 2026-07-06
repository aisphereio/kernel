package serverx

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/gatewayx"
	mwaccess "github.com/aisphereio/kernel/middleware/access"
	"github.com/aisphereio/kernel/requestx"
)

func TestServiceCatalogComposesResolvers(t *testing.T) {
	modA := ServiceModule{
		Name:            "a",
		GatewayManifest: gatewayx.Manifest{Service: "a"},
		RequestInfoResolver: func(context.Context, string, any) (requestx.Info, bool, error) {
			return requestx.Info{}, false, nil
		},
		AccessResolver: func(context.Context, string, any) (accessx.Check, bool, error) {
			return accessx.Check{}, false, nil
		},
	}
	modB := ServiceModule{
		Name:            "b",
		GatewayManifest: gatewayx.Manifest{Service: "b"},
		RequestInfoResolver: func(_ context.Context, operation string, _ any) (requestx.Info, bool, error) {
			if operation != "match" {
				return requestx.Info{}, false, nil
			}
			return requestx.Info{Operation: operation}, true, nil
		},
		AccessResolver: func(_ context.Context, operation string, _ any) (accessx.Check, bool, error) {
			if operation != "match" {
				return accessx.Check{}, false, nil
			}
			return accessx.Check{Permission: "read"}, true, nil
		},
	}

	catalog, err := NewServiceCatalog(modA, modB)
	if err != nil {
		t.Fatal(err)
	}
	info, ok, err := catalog.RequestInfoResolver(context.Background(), "match", nil)
	if err != nil || !ok || info.Operation != "match" {
		t.Fatalf("unexpected request info: info=%+v ok=%v err=%v", info, ok, err)
	}
	check, ok, err := catalog.AccessResolver(context.Background(), "match", nil)
	if err != nil || !ok || check.Permission != "read" {
		t.Fatalf("unexpected access check: check=%+v ok=%v err=%v", check, ok, err)
	}
}

func TestServiceCatalogRejectsDuplicateModules(t *testing.T) {
	resolver := func(context.Context, string, any) (requestx.Info, bool, error) {
		return requestx.Info{}, false, nil
	}
	accessResolver := func(context.Context, string, any) (accessx.Check, bool, error) {
		return accessx.Check{}, false, nil
	}
	_, err := NewServiceCatalog(
		ServiceModule{Name: "dup", GatewayManifest: gatewayx.Manifest{Service: "dup"}, RequestInfoResolver: resolver, AccessResolver: mwaccess.Resolver(accessResolver)},
		ServiceModule{Name: "dup", GatewayManifest: gatewayx.Manifest{Service: "dup"}, RequestInfoResolver: resolver, AccessResolver: mwaccess.Resolver(accessResolver)},
	)
	if err == nil {
		t.Fatal("expected duplicate module error")
	}
}
