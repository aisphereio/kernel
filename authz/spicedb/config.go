// Package spicedb contains SpiceDB adapter contracts and configuration for authz.
//
// The first production implementation should live here and implement:
//   - authz.Authorizer via CheckPermission
//   - authz.RelationshipWriter via WriteRelationships/DeleteRelationships
//   - authz.ResourceLookup via LookupResources
//   - authz.SubjectLookup via LookupSubjects
//   - authz.SchemaManager via ReadSchema/WriteSchema
package spicedb

import "time"

type Transport string

const (
	TransportGRPC Transport = "grpc"
	TransportHTTP Transport = "http"
)

type Config struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Token    string `json:"token" yaml:"token"`

	Transport Transport     `json:"transport" yaml:"transport"`
	Insecure  bool          `json:"insecure" yaml:"insecure"`
	Timeout   time.Duration `json:"timeout" yaml:"timeout"`

	// FullyConsistent is useful during early development. Production hot paths
	// should prefer at-least-as-fresh with a stored consistency token when needed.
	FullyConsistent bool `json:"fully_consistent" yaml:"fully_consistent"`
}

func (c Config) Normalized() Config {
	if c.Transport == "" {
		c.Transport = TransportGRPC
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	return c
}

const DefaultSchema = `use typechecking

definition user {}
definition service {}

definition platform {
  relation super_admin: user | service
  permission admin: user | service = super_admin
}

definition organization {
  relation platform: platform
  relation owner: user | service
  relation admin: user | service | group#member
  relation member: user | service | group#member

  permission manage: user | service = owner + admin + platform->admin
  permission read: user | service = owner + admin + member + platform->admin
}

definition group {
  relation org: organization
  relation parent: group
  relation member: user | service | group#member

  permission read: user | service = member + parent->read + org->read
}

definition application {
  relation org: organization
  relation owner: user | service
  relation admin: user | service | group#member
  relation member: user | service | group#member

  permission manage: user | service = owner + admin + org->manage
  permission read: user | service = owner + admin + member + org->read
}

definition project {
  relation org: organization
  relation owner: user | service
  relation editor: user | service | group#member
  relation viewer: user | service | group#member

  permission read: user | service = viewer + editor + owner + org->read
  permission edit: user | service = editor + owner + org->manage
  permission delete: user | service = owner + org->manage
}

definition resource {
  relation project: project
  relation owner: user | service
  relation editor: user | service | group#member
  relation viewer: user | service | group#member

  permission read: user | service = viewer + editor + owner + project->read
  permission edit: user | service = editor + owner + project->edit
  permission delete: user | service = owner + project->delete
}`
