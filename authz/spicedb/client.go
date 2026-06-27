package spicedb

import (
	"context"
	"crypto/tls"
	"io"
	"strings"

	"github.com/aisphereio/kernel/authz"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var _ authz.Service = (*Client)(nil)
var _ authz.RelationshipStore = (*Client)(nil)
var _ authz.SchemaManager = (*Client)(nil)

// Client adapts AuthZed's official authzed-go gRPC SDK to Kernel authz.
type Client struct {
	cfg    Config
	client *authzed.Client
}

func New(cfg Config, opts ...grpc.DialOption) (*Client, error) {
	cfg = cfg.Normalized()
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, authz.ErrInvalidRequest("spicedb endpoint is required")
	}

	dialOpts := make([]grpc.DialOption, 0, 4+len(opts))
	if cfg.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	}
	if cfg.Token != "" {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(bearerToken{token: cfg.Token, insecure: cfg.Insecure}))
	}
	dialOpts = append(dialOpts, opts...)

	client, err := authzed.NewClient(cfg.Endpoint, dialOpts...)
	if err != nil {
		return nil, authz.ErrBackendFailed("connect spicedb failed", err)
	}
	return &Client{cfg: cfg, client: client}, nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *Client) Check(ctx context.Context, req authz.CheckRequest) (authz.Decision, error) {
	if err := authz.ValidateCheckRequest(req); err != nil {
		return authz.Decision{}, err
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.CheckPermission(ctx, &v1.CheckPermissionRequest{
		Resource:    objectToProto(req.Resource),
		Permission:  req.Permission,
		Subject:     subjectToProto(req.Subject),
		Consistency: consistencyToProto(req.Consistency, c.cfg.FullyConsistent),
		Context:     attrsToStruct(mergeAttrs(req.SubjectAttrs, req.ResourceAttrs, req.EnvironmentAttrs)),
	})
	if err != nil {
		return authz.Decision{}, authz.ErrBackendFailed("spicedb check permission failed", err)
	}

	decision := decisionFromProto(resp)
	if resp.GetCheckedAt() != nil {
		decision.ConsistencyToken = resp.GetCheckedAt().GetToken()
	}
	return decision, nil
}

func (c *Client) BatchCheck(ctx context.Context, req authz.BatchCheckRequest) (authz.BatchCheckResult, error) {
	decisions := make([]authz.Decision, 0, len(req.Checks))
	for _, check := range req.Checks {
		if check.Consistency.Mode == "" {
			check.Consistency = req.Consistency
		}
		decision, err := c.Check(ctx, check)
		if err != nil {
			return authz.BatchCheckResult{}, err
		}
		decisions = append(decisions, decision)
	}
	return authz.BatchCheckResult{Decisions: decisions}, nil
}

func (c *Client) WriteRelationships(ctx context.Context, relationships ...authz.Relationship) (authz.WriteResult, error) {
	if len(relationships) == 0 {
		return authz.WriteResult{}, nil
	}
	updates := make([]*v1.RelationshipUpdate, 0, len(relationships))
	for _, rel := range relationships {
		if err := authz.ValidateRelationship(rel); err != nil {
			return authz.WriteResult{}, err
		}
		updates = append(updates, &v1.RelationshipUpdate{
			Operation:    v1.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: relationshipToProto(rel),
		})
	}

	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{Updates: updates})
	if err != nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("spicedb write relationships failed", err)
	}
	return authz.WriteResult{ConsistencyToken: tokenFromZed(resp.GetWrittenAt()), Written: len(updates)}, nil
}

func (c *Client) DeleteRelationships(ctx context.Context, filter authz.RelationshipFilter) (authz.WriteResult, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.DeleteRelationships(ctx, &v1.DeleteRelationshipsRequest{RelationshipFilter: filterToProto(filter)})
	if err != nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("spicedb delete relationships failed", err)
	}
	return authz.WriteResult{ConsistencyToken: tokenFromZed(resp.GetDeletedAt())}, nil
}

func (c *Client) ReadRelationships(ctx context.Context, filter authz.RelationshipFilter) ([]authz.Relationship, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	stream, err := c.client.ReadRelationships(ctx, &v1.ReadRelationshipsRequest{RelationshipFilter: filterToProto(filter)})
	if err != nil {
		return nil, authz.ErrBackendFailed("spicedb read relationships failed", err)
	}
	var out []authz.Relationship
	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, authz.ErrBackendFailed("spicedb read relationships stream failed", err)
		}
		out = append(out, relationshipFromProto(item.GetRelationship()))
	}
	return out, nil
}

func (c *Client) LookupResources(ctx context.Context, req authz.LookupResourcesRequest) (authz.LookupResourcesResult, error) {
	if req.Subject.IsZero() || strings.TrimSpace(req.ResourceType) == "" || strings.TrimSpace(req.Permission) == "" {
		return authz.LookupResourcesResult{}, authz.ErrInvalidRequest("subject, resource type and permission are required")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	stream, err := c.client.LookupResources(ctx, &v1.LookupResourcesRequest{
		ResourceObjectType: req.ResourceType,
		Permission:         req.Permission,
		Subject:            subjectToProto(req.Subject),
		Consistency:        consistencyToProto(req.Consistency, c.cfg.FullyConsistent),
		Context:            attrsToStruct(mergeAttrs(req.SubjectAttrs, req.EnvironmentAttrs)),
		OptionalLimit:      uint32FromInt(req.Limit),
		OptionalCursor:     req.Cursor,
	})
	if err != nil {
		return authz.LookupResourcesResult{}, authz.ErrBackendFailed("spicedb lookup resources failed", err)
	}
	result := authz.LookupResourcesResult{}
	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return authz.LookupResourcesResult{}, authz.ErrBackendFailed("spicedb lookup resources stream failed", err)
		}
		result.Resources = append(result.Resources, authz.ObjectRef{Type: req.ResourceType, ID: item.GetResourceObjectId()})
		result.NextCursor = item.GetAfterResultCursor()
		if item.GetLookedUpAt() != nil {
			result.ConsistencyToken = item.GetLookedUpAt().GetToken()
		}
	}
	return result, nil
}

func (c *Client) LookupSubjects(ctx context.Context, req authz.LookupSubjectsRequest) (authz.LookupSubjectsResult, error) {
	if req.Resource.IsZero() || strings.TrimSpace(req.SubjectType) == "" || strings.TrimSpace(req.Permission) == "" {
		return authz.LookupSubjectsResult{}, authz.ErrInvalidRequest("resource, subject type and permission are required")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	stream, err := c.client.LookupSubjects(ctx, &v1.LookupSubjectsRequest{
		Resource:          objectToProto(req.Resource),
		Permission:        req.Permission,
		SubjectObjectType: req.SubjectType,
		Consistency:       consistencyToProto(req.Consistency, c.cfg.FullyConsistent),
		Context:           attrsToStruct(mergeAttrs(req.ResourceAttrs, req.EnvironmentAttrs)),
		OptionalLimit:     uint32FromInt(req.Limit),
		OptionalCursor:    req.Cursor,
	})
	if err != nil {
		return authz.LookupSubjectsResult{}, authz.ErrBackendFailed("spicedb lookup subjects failed", err)
	}
	result := authz.LookupSubjectsResult{}
	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return authz.LookupSubjectsResult{}, authz.ErrBackendFailed("spicedb lookup subjects stream failed", err)
		}
		subject := item.GetSubject()
		if subject != nil {
			result.Subjects = append(result.Subjects, subjectFromProto(subject))
		}
		result.NextCursor = item.GetAfterResultCursor()
		if item.GetLookedUpAt() != nil {
			result.ConsistencyToken = item.GetLookedUpAt().GetToken()
		}
	}
	return result, nil
}

func (c *Client) ReadSchema(ctx context.Context) (authz.Schema, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.client.ReadSchema(ctx, &v1.ReadSchemaRequest{})
	if err != nil {
		return authz.Schema{}, authz.ErrBackendFailed("spicedb read schema failed", err)
	}
	return authz.Schema{Text: resp.GetSchemaText()}, nil
}

func (c *Client) WriteSchema(ctx context.Context, schema authz.Schema) error {
	if strings.TrimSpace(schema.Text) == "" {
		return authz.ErrInvalidRequest("schema text is required")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	_, err := c.client.WriteSchema(ctx, &v1.WriteSchemaRequest{Schema: schema.Text})
	if err != nil {
		return authz.ErrBackendFailed("spicedb write schema failed", err)
	}
	return nil
}

func (c *Client) ValidateSchema(ctx context.Context, schema authz.Schema) error {
	_ = ctx
	if strings.TrimSpace(schema.Text) == "" {
		return authz.ErrInvalidRequest("schema text is required")
	}
	// The stable SchemaService in authzed-go validates as part of WriteSchema.
	// Keep ValidateSchema non-mutating for now; implementation can be upgraded to
	// the developer/experimental API if Kernel later wants server-side dry-runs.
	return nil
}

func (c *Client) InstallDefaultSchema(ctx context.Context) error {
	return c.WriteSchema(ctx, authz.Schema{Text: DefaultSchema})
}
