package auditx

import (
	"context"
	"testing"
)

func TestMemoryStoreRecordAndQuery(t *testing.T) {
	store := NewMemoryStore()
	err := store.Record(context.Background(), Record{
		Action: "skill.update",
		Result: ResultSuccess,
		Actor:  Actor{SubjectID: "u_1", SubjectType: "user", OrgID: "aisphere"},
		Resource: Resource{
			Type:  "skill",
			ID:    "s_1",
			OrgID: "aisphere",
		},
		Metadata: AttributeSet{"version": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	records, err := store.Query(context.Background(), QueryFilter{ActorID: "u_1", ResourceType: "skill"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Time.IsZero() || records[0].Metadata["version"] != 2 {
		t.Fatalf("unexpected record: %#v", records[0])
	}
}
