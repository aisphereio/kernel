// Command authz-spicedb exercises Kernel authz against a real local SpiceDB.
//
// The example intentionally uses only Kernel authz contracts after bootstrapping
// the provider adapter. Application code should not call AuthZed/SpiceDB SDK
// methods directly.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authz"
	spicedbauthz "github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

type rootConfig struct {
	Log      logx.Config         `json:"log" yaml:"log"`
	SpiceDB  legacySpiceDBConfig `json:"spicedb" yaml:"spicedb"`
	Security struct {
		Authz struct {
			SpiceDB spicedbauthz.Config `json:"spicedb" yaml:"spicedb"`
		} `json:"authz" yaml:"authz"`
	} `json:"security" yaml:"security"`
	Example exampleConfig `json:"authz_example" yaml:"authz_example"`
}

type legacySpiceDBConfig struct {
	Endpoint        string `json:"endpoint" yaml:"endpoint"`
	Token           string `json:"token" yaml:"token"`
	Insecure        bool   `json:"insecure" yaml:"insecure"`
	Timeout         string `json:"timeout" yaml:"timeout"`
	FullyConsistent bool   `json:"fully_consistent" yaml:"fully_consistent"`
}

type exampleConfig struct {
	InstallSchema bool `json:"install_schema" yaml:"install_schema"`
	CleanupFirst  bool `json:"cleanup_first" yaml:"cleanup_first"`
	CleanupAfter  bool `json:"cleanup_after" yaml:"cleanup_after"`

	SubjectType       string `json:"subject_type" yaml:"subject_type"`
	SubjectID         string `json:"subject_id" yaml:"subject_id"`
	NegativeSubjectID string `json:"negative_subject_id" yaml:"negative_subject_id"`

	OrgID         string `json:"org_id" yaml:"org_id"`
	GroupID       string `json:"group_id" yaml:"group_id"`
	ApplicationID string `json:"application_id" yaml:"application_id"`
	ProjectID     string `json:"project_id" yaml:"project_id"`
	ResourceID    string `json:"resource_id" yaml:"resource_id"`
}

type options struct {
	configPath        string
	endpoint          string
	token             string
	insecure          bool
	installSchema     bool
	installSchemaSet  bool
	cleanupFirst      bool
	cleanupFirstSet   bool
	cleanupAfter      bool
	cleanupAfterSet   bool
	subjectType       string
	subjectID         string
	negativeSubjectID string
	orgID             string
	groupID           string
	applicationID     string
	projectID         string
	resourceID        string
}

type authzSuite struct {
	service       authz.Service
	relationships authz.RelationshipStore
	schema        authz.SchemaManager
	authorizer    authz.Authorizer
	audited       authz.Authorizer
	auditStore    auditx.Store
}

func main() {
	opts := parseFlags()
	ctx := context.Background()

	cfg, loadErr := loadRootConfig(opts.configPath)
	logger, closeLogger := mustLogger(cfg.Log)
	defer closeLogger()

	logger.Info("authz spicedb example starting", logx.String("config_path", opts.configPath))
	if loadErr != nil {
		logFailure(logger, "load authz spicedb example config failed", loadErr)
		os.Exit(1)
	}

	if err := run(ctx, logger, cfg, opts); err != nil {
		logFailure(logger, "authz spicedb example failed", err)
		os.Exit(1)
	}
	logger.Info("authz spicedb example finished")
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.configPath, "config", "examples/authz-spicedb/config.local.yaml", "YAML config path")
	flag.StringVar(&opts.endpoint, "endpoint", "", "SpiceDB gRPC endpoint. Overrides config")
	flag.StringVar(&opts.token, "token", "", "SpiceDB preshared key/token. Overrides config")
	flag.BoolVar(&opts.insecure, "insecure", false, "Use insecure gRPC transport. Overrides config only when true")
	flag.BoolVar(&opts.installSchema, "install-schema", false, "Install Kernel demo SpiceDB schema")
	flag.BoolVar(&opts.cleanupFirst, "cleanup-first", false, "Delete demo relationships before writing")
	flag.BoolVar(&opts.cleanupAfter, "cleanup-after", false, "Delete demo relationships after the run")
	flag.StringVar(&opts.subjectType, "subject-type", "", "Subject type. Defaults to user")
	flag.StringVar(&opts.subjectID, "subject", "", "User subject id. Use authn callback subject")
	flag.StringVar(&opts.negativeSubjectID, "negative-subject", "", "Subject id expected to be denied")
	flag.StringVar(&opts.orgID, "org", "", "Organization id")
	flag.StringVar(&opts.groupID, "group", "", "Group id")
	flag.StringVar(&opts.applicationID, "application", "", "Application id")
	flag.StringVar(&opts.projectID, "project", "", "Project id")
	flag.StringVar(&opts.resourceID, "resource", "", "Resource id")
	flag.Parse()

	// Booleans default to config values. Detect explicit true from flags without
	// forcing every omitted flag to false.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "install-schema":
			opts.installSchemaSet = true
		case "cleanup-first":
			opts.cleanupFirstSet = true
		case "cleanup-after":
			opts.cleanupAfterSet = true
		}
	})
	return opts
}

func run(ctx context.Context, logger logx.Logger, root rootConfig, opts options) error {
	spicedbCfg, err := effectiveSpiceDBConfig(root, opts)
	if err != nil {
		return err
	}
	ex := effectiveExample(root.Example, opts)
	if err := validateExample(ex); err != nil {
		return err
	}

	logger.Info("resolved spicedb authz config", spicedbConfigFields(spicedbCfg)...)
	logger.Info("resolved authz example entities", exampleFields(ex)...)

	client, err := spicedbauthz.New(spicedbCfg)
	if err != nil {
		return errorx.Wrap(err, errorx.CodeBadRequest,
			errorx.WithMessage("create spicedb kernel authz adapter failed"),
			errorx.WithMetadata("cause", err.Error()),
		)
	}
	defer func() { _ = client.Close() }()

	auditStore := auditx.NewMemoryStore()
	suite := authzSuite{
		service:       client,
		relationships: client,
		schema:        client,
		authorizer:    client,
		audited:       authz.NewAuditedAuthorizer(client, auditStore),
		auditStore:    auditStore,
	}

	if ex.InstallSchema {
		if err := installSchema(ctx, logger, suite); err != nil {
			return err
		}
	}
	if ex.CleanupFirst {
		if err := cleanupDemoRelationships(ctx, logger, suite, ex); err != nil {
			return err
		}
	}
	if err := writeDemoRelationships(ctx, logger, suite, ex); err != nil {
		return err
	}
	if err := readDemoRelationships(ctx, logger, suite, ex); err != nil {
		return err
	}
	if err := runCheckSuite(ctx, logger, suite, ex); err != nil {
		return err
	}
	if err := runLookupSuite(ctx, logger, suite, ex); err != nil {
		return err
	}
	if err := runAuditedCheck(ctx, logger, suite, ex); err != nil {
		return err
	}
	if ex.CleanupAfter {
		if err := cleanupDemoRelationships(ctx, logger, suite, ex); err != nil {
			return err
		}
	}
	return nil
}

func installSchema(ctx context.Context, logger logx.Logger, suite authzSuite) error {
	logger.Info("installing kernel demo authz schema", logx.Int("schema_len", len(spicedbauthz.DefaultSchema)))
	if err := suite.schema.ValidateSchema(ctx, authz.Schema{Text: spicedbauthz.DefaultSchema}); err != nil {
		return errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("validate spicedb schema failed"))
	}
	if err := suite.schema.WriteSchema(ctx, authz.Schema{Text: spicedbauthz.DefaultSchema}); err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("write spicedb schema failed"))
	}
	schema, err := suite.schema.ReadSchema(ctx)
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("read spicedb schema failed"))
	}
	logger.Info("spicedb schema installed", logx.Int("schema_len", len(schema.Text)))
	return nil
}

func writeDemoRelationships(ctx context.Context, logger logx.Logger, suite authzSuite, ex exampleConfig) error {
	relationships := demoRelationships(ex)
	logger.Info("writing demo relationships through kernel authz.RelationshipStore", logx.Int("relationships", len(relationships)))
	result, err := suite.relationships.WriteRelationships(ctx, relationships...)
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("write demo relationships failed"))
	}
	logger.Info("demo relationships written", logx.Int("written", result.Written), logx.String("consistency_token", result.ConsistencyToken))
	for _, rel := range relationships {
		logger.Info("relationship_written", relationshipFields(rel)...)
	}
	return nil
}

func readDemoRelationships(ctx context.Context, logger logx.Logger, suite authzSuite, ex exampleConfig) error {
	relationships, err := suite.relationships.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "resource", ResourceID: ex.ResourceID})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("read demo resource relationships failed"))
	}
	logger.Info("resource relationships read back", logx.String("resource", ex.ResourceID), logx.Int("relationships", len(relationships)))
	for _, rel := range relationships {
		logger.Info("relationship_read", relationshipFields(rel)...)
	}
	return nil
}

func runCheckSuite(ctx context.Context, logger logx.Logger, suite authzSuite, ex exampleConfig) error {
	subject := authz.SubjectRef{Type: ex.SubjectType, ID: ex.SubjectID}
	negative := authz.SubjectRef{Type: ex.SubjectType, ID: ex.NegativeSubjectID}
	checks := []struct {
		name       string
		subject    authz.SubjectRef
		resource   authz.ObjectRef
		permission string
		wantAllow  bool
	}{
		{name: "org owner can manage org", subject: subject, resource: authz.ObjectRef{Type: "organization", ID: ex.OrgID}, permission: "manage", wantAllow: true},
		{name: "group member can read application", subject: subject, resource: authz.ObjectRef{Type: "application", ID: ex.ApplicationID}, permission: "read", wantAllow: true},
		{name: "group editor can edit project", subject: subject, resource: authz.ObjectRef{Type: "project", ID: ex.ProjectID}, permission: "edit", wantAllow: true},
		{name: "project editor can edit resource through project", subject: subject, resource: authz.ObjectRef{Type: "resource", ID: ex.ResourceID}, permission: "edit", wantAllow: true},
		{name: "unknown user cannot edit resource", subject: negative, resource: authz.ObjectRef{Type: "resource", ID: ex.ResourceID}, permission: "edit", wantAllow: false},
	}

	for _, tc := range checks {
		decision, err := suite.authorizer.Check(ctx, authz.CheckRequest{Subject: tc.subject, Resource: tc.resource, Permission: tc.permission})
		if err != nil {
			return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("authz check failed"), errorx.WithMetadata("check", tc.name))
		}
		logger.Info("authz_check_result",
			logx.String("check", tc.name),
			logx.String("subject", tc.subject.String()),
			logx.String("resource", tc.resource.String()),
			logx.String("permission", tc.permission),
			logx.Bool("allowed", decision.IsAllowed()),
			logx.String("effect", string(decision.Effect)),
			logx.String("reason", decision.Reason),
			logx.Bool("expected_allowed", tc.wantAllow),
			logx.String("consistency_token", decision.ConsistencyToken),
		)
		if decision.IsAllowed() != tc.wantAllow {
			return errorx.New(errorx.CodeInternal,
				errorx.WithMessage("authz check assertion failed"),
				errorx.WithMetadata("check", tc.name),
				errorx.WithMetadata("expected_allowed", tc.wantAllow),
				errorx.WithMetadata("actual_allowed", decision.IsAllowed()),
			)
		}
	}
	return nil
}

func runLookupSuite(ctx context.Context, logger logx.Logger, suite authzSuite, ex exampleConfig) error {
	subject := authz.SubjectRef{Type: ex.SubjectType, ID: ex.SubjectID}
	resources, err := suite.service.LookupResources(ctx, authz.LookupResourcesRequest{Subject: subject, ResourceType: "resource", Permission: "edit", Limit: 20})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("lookup editable resources failed"))
	}
	logger.Info("lookup_resources_result",
		logx.String("subject", subject.String()),
		logx.String("resource_type", "resource"),
		logx.String("permission", "edit"),
		logx.Int("resources_count", len(resources.Resources)),
		logx.Any("resources", objectStrings(resources.Resources)),
		logx.String("next_cursor", resources.NextCursor),
		logx.String("consistency_token", resources.ConsistencyToken),
	)

	subjects, err := suite.service.LookupSubjects(ctx, authz.LookupSubjectsRequest{Resource: authz.ObjectRef{Type: "resource", ID: ex.ResourceID}, Permission: "edit", SubjectType: ex.SubjectType, Limit: 20})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("lookup subjects failed"))
	}
	logger.Info("lookup_subjects_result",
		logx.String("resource", authz.ObjectRef{Type: "resource", ID: ex.ResourceID}.String()),
		logx.String("permission", "edit"),
		logx.String("subject_type", ex.SubjectType),
		logx.Int("subjects_count", len(subjects.Subjects)),
		logx.Any("subjects", subjectStrings(subjects.Subjects)),
		logx.String("next_cursor", subjects.NextCursor),
		logx.String("consistency_token", subjects.ConsistencyToken),
	)
	return nil
}

func runAuditedCheck(ctx context.Context, logger logx.Logger, suite authzSuite, ex exampleConfig) error {
	decision, err := suite.audited.Check(ctx, authz.CheckRequest{Subject: authz.SubjectRef{Type: ex.SubjectType, ID: ex.SubjectID}, Resource: authz.ObjectRef{Type: "resource", ID: ex.ResourceID}, Permission: "edit"})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("audited authz check failed"))
	}
	records, err := suite.auditStore.Query(ctx, auditx.QueryFilter{ActorID: ex.SubjectID, Action: "authz.check"})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("query audit records failed"))
	}
	logger.Info("audited_authz_check_result",
		logx.Bool("allowed", decision.IsAllowed()),
		logx.String("effect", string(decision.Effect)),
		logx.Int("audit_records", len(records)),
	)
	for _, record := range records {
		logger.Info("audit_record",
			logx.String("id", record.ID),
			logx.String("actor_id", record.Actor.SubjectID),
			logx.String("actor_type", record.Actor.SubjectType),
			logx.String("action", record.Action),
			logx.String("resource_type", record.Resource.Type),
			logx.String("resource_id", record.Resource.ID),
			logx.String("result", record.Result),
			logx.String("severity", record.Severity),
			logx.String("reason", record.Reason),
		)
	}
	return nil
}

func cleanupDemoRelationships(ctx context.Context, logger logx.Logger, suite authzSuite, ex exampleConfig) error {
	filters := []authz.RelationshipFilter{
		{ResourceType: "resource", ResourceID: ex.ResourceID},
		{ResourceType: "project", ResourceID: ex.ProjectID},
		{ResourceType: "application", ResourceID: ex.ApplicationID},
		{ResourceType: "group", ResourceID: ex.GroupID},
		{ResourceType: "organization", ResourceID: ex.OrgID},
	}
	for _, filter := range filters {
		result, err := suite.relationships.DeleteRelationships(ctx, filter)
		if err != nil {
			return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("cleanup demo relationships failed"), errorx.WithMetadata("resource_type", filter.ResourceType), errorx.WithMetadata("resource_id", filter.ResourceID))
		}
		logger.Info("cleanup_relationships", logx.String("resource_type", filter.ResourceType), logx.String("resource_id", filter.ResourceID), logx.Int("deleted", result.Deleted), logx.String("consistency_token", result.ConsistencyToken))
	}
	return nil
}

func demoRelationships(ex exampleConfig) []authz.Relationship {
	subject := authz.SubjectRef{Type: ex.SubjectType, ID: ex.SubjectID}
	groupMembers := authz.SubjectRef{Type: "group", ID: ex.GroupID, Relation: "member"}
	return []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "organization", ID: ex.OrgID}, Relation: "owner", Subject: subject},
		{Resource: authz.ObjectRef{Type: "group", ID: ex.GroupID}, Relation: "org", Subject: authz.SubjectRef{Type: "organization", ID: ex.OrgID}},
		{Resource: authz.ObjectRef{Type: "group", ID: ex.GroupID}, Relation: "member", Subject: subject},
		{Resource: authz.ObjectRef{Type: "application", ID: ex.ApplicationID}, Relation: "org", Subject: authz.SubjectRef{Type: "organization", ID: ex.OrgID}},
		{Resource: authz.ObjectRef{Type: "application", ID: ex.ApplicationID}, Relation: "member", Subject: groupMembers},
		{Resource: authz.ObjectRef{Type: "project", ID: ex.ProjectID}, Relation: "org", Subject: authz.SubjectRef{Type: "organization", ID: ex.OrgID}},
		{Resource: authz.ObjectRef{Type: "project", ID: ex.ProjectID}, Relation: "editor", Subject: groupMembers},
		{Resource: authz.ObjectRef{Type: "resource", ID: ex.ResourceID}, Relation: "project", Subject: authz.SubjectRef{Type: "project", ID: ex.ProjectID}},
	}
}

func effectiveSpiceDBConfig(root rootConfig, opts options) (spicedbauthz.Config, error) {
	cfg := root.Security.Authz.SpiceDB
	if hasLegacySpiceDBConfig(root.SpiceDB) {
		cfg.Endpoint = firstNonEmpty(root.SpiceDB.Endpoint, cfg.Endpoint)
		cfg.Token = firstNonEmpty(root.SpiceDB.Token, cfg.Token)
		cfg.Insecure = root.SpiceDB.Insecure || cfg.Insecure
		cfg.FullyConsistent = root.SpiceDB.FullyConsistent || cfg.FullyConsistent
		if root.SpiceDB.Timeout != "" {
			d, err := time.ParseDuration(root.SpiceDB.Timeout)
			if err != nil {
				return cfg, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("invalid spicedb.timeout"))
			}
			cfg.Timeout = d
		}
	}
	if opts.endpoint != "" {
		cfg.Endpoint = opts.endpoint
	}
	if opts.token != "" {
		cfg.Token = opts.token
	}
	if opts.insecure {
		cfg.Insecure = true
	}
	return cfg.Normalized(), nil
}

func effectiveExample(ex exampleConfig, opts options) exampleConfig {
	if opts.installSchemaSet {
		ex.InstallSchema = opts.installSchema
	}
	if opts.cleanupFirstSet {
		ex.CleanupFirst = opts.cleanupFirst
	}
	if opts.cleanupAfterSet {
		ex.CleanupAfter = opts.cleanupAfter
	}
	ex.SubjectType = firstNonEmpty(opts.subjectType, ex.SubjectType, authz.SubjectTypeUser)
	ex.SubjectID = firstNonEmpty(opts.subjectID, ex.SubjectID)
	ex.NegativeSubjectID = firstNonEmpty(opts.negativeSubjectID, ex.NegativeSubjectID, "not-a-member")
	ex.OrgID = firstNonEmpty(opts.orgID, ex.OrgID, "aisphere")
	ex.GroupID = firstNonEmpty(opts.groupID, ex.GroupID, "aisphere-dev")
	ex.ApplicationID = firstNonEmpty(opts.applicationID, ex.ApplicationID, "aisphere")
	ex.ProjectID = firstNonEmpty(opts.projectID, ex.ProjectID, "demo-project")
	ex.ResourceID = firstNonEmpty(opts.resourceID, ex.ResourceID, "agent-demo")
	return ex
}

func validateExample(ex exampleConfig) error {
	if strings.TrimSpace(ex.SubjectID) == "" {
		return authz.ErrInvalidRequest("authz_example.subject_id is required; use the subject printed by examples/authn-casdoor")
	}
	if strings.TrimSpace(ex.NegativeSubjectID) == "" {
		return authz.ErrInvalidRequest("authz_example.negative_subject_id is required")
	}
	if strings.TrimSpace(ex.OrgID) == "" || strings.TrimSpace(ex.GroupID) == "" || strings.TrimSpace(ex.ApplicationID) == "" || strings.TrimSpace(ex.ProjectID) == "" || strings.TrimSpace(ex.ResourceID) == "" {
		return authz.ErrInvalidRequest("org, group, application, project and resource ids are required")
	}
	return nil
}

func hasLegacySpiceDBConfig(cfg legacySpiceDBConfig) bool {
	return firstNonEmpty(cfg.Endpoint, cfg.Token, cfg.Timeout) != "" || cfg.Insecure || cfg.FullyConsistent
}

func loadRootConfig(path string) (rootConfig, error) {
	if path == "" {
		return rootConfig{}, errorx.BadRequest(errorx.CodeBadRequest, "config path is required")
	}
	cfg := configx.New(configx.WithSource(file.NewSource(path)))
	if err := cfg.Load(); err != nil {
		return rootConfig{}, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("load config failed"))
	}
	defer cfg.Close()
	var root rootConfig
	if err := cfg.Scan(&root); err != nil {
		return rootConfig{}, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("scan config failed"))
	}
	return root, nil
}

func logFailure(logger logx.Logger, message string, err error) {
	fields := []logx.Field{logx.Err(err), logx.String("error_code", errorx.CodeOf(err).String()), logx.String("safe_message", errorx.MessageOf(err))}
	if cause := errorx.CauseOf(err); cause != nil {
		fields = append(fields, logx.String("cause", cause.Error()))
	}
	if md := errorx.MetadataOf(err); len(md) > 0 {
		fields = append(fields, logx.Any("metadata", md))
	}
	logger.Error(message, fields...)
}

func mustLogger(cfg logx.Config) (logx.Logger, func()) {
	if cfg.ServiceName == "" {
		cfg = logx.DefaultConfig("dev")
		cfg.ServiceName = "authz-spicedb-example"
		cfg.Level = logx.DebugLevel.String()
		cfg.Format = logx.FormatConsole
		cfg.Output = string(logx.OutputStdout)
	}
	logger, _, err := logx.New(cfg)
	if err != nil {
		fallback := logx.Noop()
		fmt.Fprintf(os.Stderr, "failed to initialize logx: %v\n", err)
		return fallback, func() {}
	}
	return logger, func() { _ = logger.Sync() }
}

func spicedbConfigFields(cfg spicedbauthz.Config) []logx.Field {
	return []logx.Field{
		logx.String("endpoint", cfg.Endpoint),
		logx.Bool("has_token", cfg.Token != ""),
		logx.Bool("insecure", cfg.Insecure),
		logx.Duration("timeout", cfg.Timeout),
		logx.Bool("fully_consistent", cfg.FullyConsistent),
	}
}

func exampleFields(ex exampleConfig) []logx.Field {
	return []logx.Field{
		logx.Bool("install_schema", ex.InstallSchema),
		logx.Bool("cleanup_first", ex.CleanupFirst),
		logx.Bool("cleanup_after", ex.CleanupAfter),
		logx.String("subject", authz.SubjectRef{Type: ex.SubjectType, ID: ex.SubjectID}.String()),
		logx.String("negative_subject", authz.SubjectRef{Type: ex.SubjectType, ID: ex.NegativeSubjectID}.String()),
		logx.String("org", ex.OrgID),
		logx.String("group", ex.GroupID),
		logx.String("application", ex.ApplicationID),
		logx.String("project", ex.ProjectID),
		logx.String("resource", ex.ResourceID),
	}
}

func relationshipFields(rel authz.Relationship) []logx.Field {
	return []logx.Field{
		logx.String("resource", rel.Resource.String()),
		logx.String("relation", rel.Relation),
		logx.String("subject", rel.Subject.String()),
		logx.String("caveat", rel.CaveatName),
		logx.Time("expires_at", rel.ExpiresAt),
	}
}

func objectStrings(objects []authz.ObjectRef) []string {
	out := make([]string, 0, len(objects))
	for _, obj := range objects {
		out = append(out, obj.String())
	}
	return out
}

func subjectStrings(subjects []authz.SubjectRef) []string {
	out := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		out = append(out, subject.String())
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
