package servicecontextx_test

import (
	"testing"

	"github.com/aisphereio/kernel/servicecontextx"
)

type fakeSkillClient struct{ name string }

func TestClientRegistry(t *testing.T) {
	sc := servicecontextx.New()
	client := &fakeSkillClient{name: "skill"}
	sc.MustPutClient(servicecontextx.ClientRef{
		Name:   "skill",
		Kind:   servicecontextx.ClientKindGRPC,
		Target: "skill-service:9000",
		Client: client,
	})

	ref := servicecontextx.MustClient(sc, "skill")
	if ref.Kind != servicecontextx.ClientKindGRPC || ref.Target != "skill-service:9000" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	got := servicecontextx.MustTypedClient[*fakeSkillClient](sc, "skill")
	if got != client {
		t.Fatalf("got %#v, want %#v", got, client)
	}
}
