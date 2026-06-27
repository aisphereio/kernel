package logx

// AuditHint is intentionally not an audit record. It only leaves an operational
// log breadcrumb saying an auditable operation happened. Compliance-grade
// records must go through auditx.
type AuditHint struct {
	Action       string
	ActorID      string
	ResourceType string
	ResourceID   string
	Result       string
	Fields       []Field
}

func LogAuditHint(logger Logger, hint AuditHint) {
	if logger == nil {
		logger = Noop()
	}
	fields := []Field{
		Event("audit_hint"),
		String("action", hint.Action),
		String("actor_id", hint.ActorID),
		String("resource_type", hint.ResourceType),
		String("resource_id", hint.ResourceID),
		String("result", hint.Result),
	}
	fields = append(fields, hint.Fields...)
	logger.Info("audit hint", fields...)
}
