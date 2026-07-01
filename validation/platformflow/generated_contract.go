package platformflow

import (
	"context"

	"github.com/aisphereio/kernel/accessx"
	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/requestx"
	"github.com/aisphereio/kernel/serverx"
)

// IAMServiceGatewayManifest is the generated-equivalent output of
// protoc-gen-go-gateway for validation/iamservice/api/iam/v1/iam.proto.
func IAMServiceGatewayManifest() gatewayx.Manifest {
	return gatewayx.Manifest{Service: "iam-service", Namespace: "aisphere", Routes: []gatewayx.GatewayRoute{{
		ID: "iam.login", Method: "POST", Path: "/v1/iam/login",
		Upstream: gatewayx.UpstreamRef{Service: "iam-service", Namespace: "aisphere", Protocol: "grpc", Operation: "/aisphere.iam.v1.IAMService/Login"},
		Gateway:  gatewayx.GatewayPolicy{Exposure: accessv1.Exposure_PUBLIC, AuthnMode: gatewayx.AuthnModeNone, ForwardAuthorization: false},
	}}}
}

// SkillServiceGatewayManifest is the generated-equivalent output of
// protoc-gen-go-gateway for a skill service proto.
func SkillServiceGatewayManifest() gatewayx.Manifest {
	return gatewayx.Manifest{Service: "skill-service", Namespace: "aisphere", Routes: []gatewayx.GatewayRoute{{
		ID: "skill.get", Method: "GET", Path: "/v1/skills/{id}",
		Upstream: gatewayx.UpstreamRef{Service: "skill-service", Namespace: "aisphere", Protocol: "grpc", Operation: "/aisphere.skill.v1.SkillService/GetSkill"},
		Gateway:  gatewayx.GatewayPolicy{Exposure: accessv1.Exposure_AUTHORIZED, AuthnMode: gatewayx.AuthnModePassive, ForwardAuthorization: true},
	}}}
}

// IAMServiceKernelModule is generated-equivalent output of protoc-gen-go-kernel.
func IAMServiceKernelModule() serverx.ServiceModule {
	return serverx.ServiceModule{
		Name:            "iam-service",
		GatewayManifest: IAMServiceGatewayManifest(),
		RequestInfoResolver: func(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
			_ = ctx
			_ = operation
			_ = req
			return requestx.Info{Direction: requestx.DirectionServer, Protocol: requestx.ProtocolGRPC, Service: "aisphere.iam.v1.IAMService", Method: "Login", Operation: "/aisphere.iam.v1.IAMService/Login", Exposure: accessv1.Exposure_PUBLIC, Action: "iam.login", TargetService: "iam-service"}.Normalize(), true, nil
		},
		AccessResolver: func(context.Context, string, any) (accessx.Check, bool, error) { return accessx.Check{}, false, nil },
	}
}

// SkillServiceKernelModule is generated-equivalent output of protoc-gen-go-kernel.
func SkillServiceKernelModule() serverx.ServiceModule {
	return serverx.ServiceModule{
		Name:                "skill-service",
		GatewayManifest:     SkillServiceGatewayManifest(),
		RequestInfoResolver: SkillServiceRequestInfoResolver,
		AccessResolver:      SkillServiceAccessResolver,
	}
}

// SkillServiceRequestInfoResolver is the generated-equivalent output of
// protoc-gen-go-authz. It makes SkillService the authoritative authn/authz
// enforcement point after Gateway performs passive token relay.
func SkillServiceRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	_ = ctx
	switch req.(type) {
	case GetSkillRequest:
		return requestx.Info{Direction: requestx.DirectionServer, Protocol: requestx.ProtocolGRPC, Service: "aisphere.skill.v1.SkillService", Method: "GetSkill", Operation: "/aisphere.skill.v1.SkillService/GetSkill", Exposure: accessv1.Exposure_AUTHORIZED, Action: "skill.get", TargetService: "skill-service"}.Normalize(), true, nil
	default:
		return requestx.Info{}, false, nil
	}
}

// SkillServiceAccessResolver maps proto access policy to accessx.Check. The
// Check is executed by accessx.Guard inside SkillService, not by Gateway.
func SkillServiceAccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
	_ = ctx
	r, ok := req.(GetSkillRequest)
	if !ok {
		return accessx.Check{}, false, nil
	}
	return accessx.Check{Permission: "get", Resource: authz.ObjectRef{Type: "skill", ID: r.ID}, AuditAction: "skill.get"}, true, nil
}

// IAMServiceGatewayBindLogin is generated-equivalent HTTP -> gRPC request binding.
func IAMServiceGatewayBindLogin(req gatewayx.DispatchRequest, match gatewayx.RouteMatch) (LoginRequest, error) {
	_ = match
	if v, ok := req.Body.(LoginRequest); ok {
		return v, nil
	}
	return LoginRequest{}, nil
}

// SkillServiceGatewayBindGetSkill is generated-equivalent HTTP -> gRPC request binding.
func SkillServiceGatewayBindGetSkill(req gatewayx.DispatchRequest, match gatewayx.RouteMatch) (GetSkillRequest, error) {
	_ = req
	id := match.Params["id"]
	if id == "" {
		return GetSkillRequest{}, errorx.BadRequest("SKILL_ID_REQUIRED", "skill id required")
	}
	return GetSkillRequest{ID: id}, nil
}

// RegisterPlatformGatewayInvokers is generated-equivalent Gateway -> service gRPC binding registration.
func RegisterPlatformGatewayInvokers(registry *gatewayx.InvokerRegistry, iam iamService, skill *skillService, skillHandler middleware.Handler) error {
	if err := registry.Register("/aisphere.iam.v1.IAMService/Login", gatewayx.UnaryInvoker(IAMServiceGatewayBindLogin, iam.Login)); err != nil {
		return err
	}
	return registry.Register("/aisphere.skill.v1.SkillService/GetSkill", gatewayx.UnaryInvoker(SkillServiceGatewayBindGetSkill, func(ctx context.Context, req GetSkillRequest) (Skill, error) {
		out, err := skillHandler(ctx, req)
		if err != nil {
			return Skill{}, err
		}
		return out.(Skill), nil
	}))
}
