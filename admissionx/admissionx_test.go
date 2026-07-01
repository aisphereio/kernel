package admissionx

import (
	"context"
	"errors"
	"testing"
)

type obj struct{ Name string }

func TestAdmissionMutateThenValidate(t *testing.T) {
	chain := New([]MutatingPlugin{MutatingPluginFunc{PluginName: "default-name", Fn: func(ctx context.Context, a Attributes) (any, error) {
		o := a.Object.(obj)
		if o.Name == "" {
			o.Name = "default"
		}
		return o, nil
	}}}, []ValidatingPlugin{ValidatingPluginFunc{PluginName: "validate-name", Fn: func(ctx context.Context, a Attributes) error {
		if a.Object.(obj).Name == "" {
			return errors.New("name required")
		}
		return nil
	}}})
	got, err := chain.Admit(context.Background(), obj{})
	if err != nil {
		t.Fatal(err)
	}
	if got.(obj).Name != "default" {
		t.Fatalf("unexpected object: %#v", got)
	}
}

func TestAdmissionValidateRejects(t *testing.T) {
	chain := New(nil, []ValidatingPlugin{ValidatingPluginFunc{PluginName: "deny", Fn: func(ctx context.Context, a Attributes) error { return errors.New("no") }}})
	if _, err := chain.Admit(context.Background(), obj{}); err == nil {
		t.Fatal("expected error")
	}
}
