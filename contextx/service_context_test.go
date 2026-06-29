package contextx_test

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/contextx"
)

type fakeDep struct{ value string }

func TestServiceContextRoundTrip(t *testing.T) {
	sc := contextx.MustNewServiceContext()
	dep := &fakeDep{value: "ok"}
	contextx.MustPutDependency(sc, dep)

	ctx := contextx.WithServiceContext(context.Background(), sc)
	got := contextx.MustGetDependencyFromContext[*fakeDep](ctx)
	if got != dep {
		t.Fatalf("got %#v, want %#v", got, dep)
	}
}
