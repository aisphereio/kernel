package servicecontextx_test

import (
	"reflect"
	"testing"

	"github.com/aisphereio/kernel/servicecontextx"
)

type dep struct{ Name string }

func TestContextTypedAndNamedDependencies(t *testing.T) {
	ctx := servicecontextx.New()
	ctx.MustPut(&dep{Name: "typed"})
	ctx.MustPutAs("writer", &dep{Name: "named"})

	got := servicecontextx.MustGet[*dep](ctx)
	if got.Name != "typed" {
		t.Fatalf("typed dep=%q", got.Name)
	}

	named := servicecontextx.MustGetAs[*dep](ctx, "writer")
	if named.Name != "named" {
		t.Fatalf("named dep=%q", named.Name)
	}
}

func TestContextCloseReverseOrder(t *testing.T) {
	ctx := servicecontextx.New()
	var order []int
	ctx.OnClose(func() error { order = append(order, 1); return nil })
	ctx.OnClose(func() error { order = append(order, 2); return nil })
	if err := ctx.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !reflect.DeepEqual(order, []int{2, 1}) {
		t.Fatalf("close order=%v", order)
	}
}
