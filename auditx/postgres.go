package auditx

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aisphereio/kernel/dbx"
)

// AuditLogModel is the GORM model for the iam_audit_logs table.
type AuditLogModel struct {
	ID             string          `gorm:"column:id;primaryKey"`
	ActorType      string          `gorm:"column:actor_type;not null"`
	ActorID        string          `gorm:"column:actor_id;not null"`
	TechnicalActor string          `gorm:"column:technical_actor"`
	Action         string          `gorm:"column:action;not null"`
	ResourceType   string          `gorm:"column:resource_type"`
	ResourceID     string          `gorm:"column:resource_id"`
	Result         string          `gorm:"column:result;not null"`
	Reason         string          `gorm:"column:reason"`
	RequestID      string          `gorm:"column:request_id"`
	TraceID        string          `gorm:"column:trace_id"`
	IP             string          `gorm:"column:ip"`
	UserAgent      string          `gorm:"column:user_agent"`
	Metadata       json.RawMessage `gorm:"column:metadata;type:jsonb"`
	CreatedAt      time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (AuditLogModel) TableName() string { return "iam_audit_logs" }

// PostgresStore implements auditx.Store backed by PostgreSQL via dbx.
type PostgresStore struct {
	db  dbx.DB
	now func() time.Time
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db dbx.DB) *PostgresStore {
	return &PostgresStore{db: db, now: time.Now}
}

// EnsureTable creates the audit_logs table if it does not exist.
func (s *PostgresStore) EnsureTable(ctx context.Context) error {
	return s.db.AutoMigrate(ctx, &AuditLogModel{})
}

func (s *PostgresStore) Record(ctx context.Context, record Record) error {
	if err := Validate(record); err != nil {
		return err
	}
	now := s.now()
	record = record.WithTime(now)
	metadata, _ := json.Marshal(record.Metadata)
	model := &AuditLogModel{
		ID:             record.ID,
		ActorType:      record.Actor.SubjectType,
		ActorID:        record.Actor.SubjectID,
		TechnicalActor: record.Actor.Name,
		Action:         record.Action,
		ResourceType:   record.Resource.Type,
		ResourceID:     record.Resource.ID,
		Result:         record.Result,
		Reason:         record.Reason,
		RequestID:      record.RequestID,
		TraceID:        record.TraceID,
		IP:             record.ClientIP,
		UserAgent:      record.UserAgent,
		Metadata:       metadata,
		CreatedAt:      now,
	}
	return s.db.Create(ctx, model)
}

func (s *PostgresStore) Query(ctx context.Context, filter QueryFilter) ([]Record, error) {
	var models []AuditLogModel
	query := s.db.GORM(ctx).Model(&AuditLogModel{})
	if filter.ActorID != "" {
		query = query.Where("actor_id = ?", filter.ActorID)
	}
	if filter.ActorType != "" {
		query = query.Where("actor_type = ?", filter.ActorType)
	}
	if filter.ResourceType != "" {
		query = query.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID != "" {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Result != "" {
		query = query.Where("result = ?", filter.Result)
	}
	if !filter.From.IsZero() {
		query = query.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		query = query.Where("created_at < ?", filter.To)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	query = query.Order("created_at DESC")
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(models))
	for _, m := range models {
		out = append(out, modelToRecord(m))
	}
	return out, nil
}

func modelToRecord(m AuditLogModel) Record {
	meta := make(AttributeSet)
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &meta)
	}
	return Record{
		ID:     m.ID,
		Time:   m.CreatedAt,
		Action: m.Action,
		Result: m.Result,
		Reason: m.Reason,
		Actor: Actor{
			SubjectID:   m.ActorID,
			SubjectType: m.ActorType,
			Name:        m.TechnicalActor,
		},
		Resource: Resource{
			Type: m.ResourceType,
			ID:   m.ResourceID,
		},
		RequestID: m.RequestID,
		TraceID:   m.TraceID,
		ClientIP:  m.IP,
		UserAgent: m.UserAgent,
		Metadata:  meta,
	}
}

func (s *PostgresStore) currentTime() time.Time {
		if s.now != nil {
			return s.now()
		}
		return time.Now().UTC()
	}