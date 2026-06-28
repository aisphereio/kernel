package spicedb

import (
	"testing"
	"time"

	"github.com/aisphereio/kernel/authz"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
)

func TestRelationshipExpirationRoundTrip(t *testing.T) {
	expiresAt := time.Date(2026, 6, 27, 12, 30, 0, 0, time.UTC)
	rel := authz.Relationship{
		Resource:  authz.ObjectRef{Type: "document", ID: "doc_1"},
		Relation:  "viewer",
		Subject:   authz.SubjectRef{Type: "user", ID: "u_1"},
		ExpiresAt: expiresAt,
	}

	protoRel := relationshipToProto(rel)
	if protoRel.GetOptionalExpiresAt() == nil {
		t.Fatal("expected optional_expires_at to be set")
	}

	got := relationshipFromProto(protoRel)
	if !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("ExpiresAt = %s, want %s", got.ExpiresAt, expiresAt)
	}
}

func TestCursorConversion(t *testing.T) {
	if got := cursorToProto(""); got != nil {
		t.Fatalf("cursorToProto empty = %v, want nil", got)
	}
	if got := cursorToProto("  "); got != nil {
		t.Fatalf("cursorToProto blank = %v, want nil", got)
	}

	protoCursor := cursorToProto("cursor_1")
	if protoCursor == nil || protoCursor.GetToken() != "cursor_1" {
		t.Fatalf("cursorToProto token = %v, want cursor_1", protoCursor)
	}
	if got := cursorFromProto(protoCursor); got != "cursor_1" {
		t.Fatalf("cursorFromProto = %q, want cursor_1", got)
	}
	if got := cursorFromProto(nil); got != "" {
		t.Fatalf("cursorFromProto nil = %q, want empty", got)
	}
}

func TestResolvedSubjectFromProto(t *testing.T) {
	got := resolvedSubjectFromProto("user", &v1.ResolvedSubject{SubjectObjectId: "u_1"})
	want := authz.SubjectRef{Type: "user", ID: "u_1"}
	if got != want {
		t.Fatalf("resolvedSubjectFromProto = %#v, want %#v", got, want)
	}
}
