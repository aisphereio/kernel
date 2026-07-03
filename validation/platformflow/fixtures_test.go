//go:build platformflow_validation
// +build platformflow_validation

package platformflow

import (
	"context"

	"github.com/aisphereio/kernel/accessx"
	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/requestx"
	"github.com/aisphereio/kernel/serverx"
)

type LoginRequest struct {
	Username string
	Password string
}

type LoginReply struct {
	AccessToken string
}

type GetSkillRequest struct {
	ID string
}

type Skill struct {
	ID   string
	Name string
}

type iamService struct{}

func (iamService) Login(_ context.Context, req LoginRequest) (LoginReply, error) {
	switch req.Username {
	case "alice":
		return LoginReply{AccessToken: "alice-token"}, nil
	case "bob":
		return LoginReply{AccessToken: "bob-token"}, nil
	default:
		return LoginReply{}, authn.ErrUnauthenticated("invalid demo user")
	}
}

type fakeAuthnProvider struct {
	tokens map[string]authn.Principal
}

func (p fakeAuthnProvider) Authenticate(_ context.Context, cred authn.Credential) (authn.Principal, error) {
	principal, ok := p.tokens[cred.Token]
	if !ok {
		return authn.Principal{}, authn.ErrUnauthenticated("invalid demo token")
	}
	principal.Provider = "casdoor-demo"
	principal.AuthMethod = authn.AuthMethodJWT
	return principal, nil
}

func (p fakeAuthnProvider) ExchangeCode(context.Context, authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return authn.TokenSet{}, authn.Principal{}, authn.ErrIdentityBackendFailed("not implemented in demo", nil)
}

func (p fakeAuthnProvider) RefreshToken(context.Context, authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return authn.TokenSet{}, authn.ErrIdentityBackendFailed("not implemented in demo", nil)
}

func (p fakeAuthnProvider) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return p.Authenticate(ctx, authn.Credential{Scheme: authn.CredentialBearer, Token: req.Token})
}

func (p fakeAuthnProvider) RevokeToken(context.Context, authn.RevokeTokenRequest) error { return nil }

func (p fakeAuthnProvider) BuildLoginURL(context.Context, authn.LoginURLRequest) (authn.LoginURL, error) {
	return authn.LoginURL{URL: "https://casdoor.example.com/login", Provider: "casdoor"}, nil
}

func (p fakeAuthnProvider) HandleCallback(context.Context, authn.CallbackRequest) (authn.CallbackResult, error) {
	return authn.CallbackResult{Provider: "casdoor"}, nil
}

type skillService struct {
	skills map[string]Skill
	called int
}

func (s *skillService) GetSkill(_ context.Context, req GetSkillRequest) (Skill, error) {
	s.called++
	skill, ok := s.skills[req.ID]
	if !ok {
		return Skill{}, errorx.NotFound("SKILL_NOT_FOUND", "skill not found")
	}
	return skill, nil
}

func IAMServiceKernelModule() serverx.ServiceModule {
	return serverx.ServiceModule{
		Name:                "iam-service",
		GatewayManifest:     IAMServiceGatewayManifest(),
		RequestInfoResolver: iamRequestInfoResolver,
		AccessResolver:      iamAccessResolver,
	}
}

func SkillServiceKernelModule() serverx.ServiceModule {
	return serverx.ServiceModule{
		Name:                "skill-service",
		GatewayManifest:     SkillServiceGatewayManifest(),
		RequestInfoResolver: skillRequestInfoResolver,
		AccessResolver:      skillAccessResolver,
	}
}

func IAMServiceGatewayManifest() gatewayx.Manifest {
	return gatewayx.Manifest{
		Service:   "iam-service",
		Namespace: "aisphere",
		Routes: []gatewayx.GatewayRoute{{
			ID:     "iam.login",
			Method: "POST",
			Path:   "/v1/iam/login",
			Upstream: gatewayx.UpstreamRef{
				Service: "iam-service", Namespace: "aisphere", Protocol: "grpc", Operation: "/aisphere.iam.v1.IAMService/Login",
			},
			Gateway: gatewayx.GatewayPolicy{Exposure: accessv1.Exposure_PUBLIC, AuthnMode: gatewayx.AuthnModeNone},
		}},
	}
}

func SkillServiceGatewayManifest() gatewayx.Manifest {
	return gatewayx.Manifest{
		Service:   "skill-service",
		Namespace: "aisphere",
		Routes: []gatewayx.GatewayRoute{{
			ID:     "skill.get.skill",
			Method: "GET",
			Path:   "/v1/skills/{id}",
			Upstream: gatewayx.UpstreamRef{
				Service: "skill-service", Namespace: "aisphere", Protocol: "grpc", Operation: "/aisphere.skill.v1.SkillService/GetSkill",
			},
			Gateway: gatewayx.GatewayPolicy{Exposure: accessv1.Exposure_AUTHORIZED, AuthnMode: gatewayx.AuthnModePassive, ForwardAuthorization: true},
		}},
	}
}

func IAMServiceGatewayBindLogin(req gatewayx.DispatchRequest, _ gatewayx.RouteMatch) (*LoginRequest, error) {
	out := &LoginRequest{}
	if v, ok := req.Body.(*LoginRequest); ok && v != nil {
		out = v
	}
	if v, ok := req.Body.(LoginRequest); ok {
		out = &v
	}
	return out, nil
}

func SkillServiceGatewayBindGetSkill(req gatewayx.DispatchRequest, match gatewayx.RouteMatch) (*GetSkillRequest, error) {
	out := &GetSkillRequest{}
	if v, ok := req.Body.(*GetSkillRequest); ok && v != nil {
		out = v
	}
	if v, ok := req.Body.(GetSkillRequest); ok {
		out = &v
	}
	if v := match.Params["id"]; v != "" {
		out.ID = v
	}
	return out, nil
}

func iamRequestInfoResolver(_ context.Context, _ string, req any) (requestx.Info, bool, error) {
	if _, ok := req.(LoginRequest); !ok {
		return requestx.Info{}, false, nil
	}
	return requestx.Info{
		Operation: "/aisphere.iam.v1.IAMService/Login",
		Service:   "aisphere.iam.v1.IAMService",
		Method:    "Login",
		Exposure:  accessv1.Exposure_PUBLIC,
		Action:    "iam.login",
	}, true, nil
}

func iamAccessResolver(context.Context, string, any) (accessx.Check, bool, error) {
	return accessx.Check{}, false, nil
}

func skillRequestInfoResolver(_ context.Context, _ string, req any) (requestx.Info, bool, error) {
	r, ok := req.(GetSkillRequest)
	if !ok {
		return requestx.Info{}, false, nil
	}
	return requestx.Info{
		Operation: "/aisphere.skill.v1.SkillService/GetSkill",
		Service:   "aisphere.skill.v1.SkillService",
		Method:    "GetSkill",
		Exposure:  accessv1.Exposure_AUTHORIZED,
		Action:    "skill.get",
		Resource:  "skill:" + r.ID,
	}, true, nil
}

func skillAccessResolver(_ context.Context, _ string, req any) (accessx.Check, bool, error) {
	r, ok := req.(GetSkillRequest)
	if !ok {
		return accessx.Check{}, false, nil
	}
	return accessx.Check{
		Permission:  "get",
		Resource:    authz.ObjectRef{Type: "skill", ID: r.ID},
		AuditAction: "skill.get",
	}, true, nil
}
