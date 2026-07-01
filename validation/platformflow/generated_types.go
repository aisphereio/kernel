package platformflow

import "context"

// LoginRequest is the validation DTO used by generated gateway and gRPC code.
type LoginRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// LoginReply is the validation DTO returned by the IAM login flow.
type LoginReply struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Subject     string `json:"subject,omitempty"`
}

// GetSkillRequest is the validation DTO used by the skill service flow.
type GetSkillRequest struct {
	ID string `json:"id,omitempty"`
}

// Skill is the validation DTO returned by the skill service flow.
type Skill struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// iamService is the generated-equivalent IAM server contract used by the
// platformflow validation package.
type iamService interface {
	Login(context.Context, LoginRequest) (LoginReply, error)
}

// skillService is the generated-equivalent skill service used by validation
// tests. The gateway path normally exercises the middleware.Handler adapter,
// but keeping this concrete type makes the package self-contained for go list,
// go test, and govulncheck.
type skillService struct{}

func (skillService) GetSkill(_ context.Context, req GetSkillRequest) (Skill, error) {
	return Skill{ID: req.ID, Name: req.ID}, nil
}
