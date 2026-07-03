//go:build platformflow_validation
// +build platformflow_validation

package platformflow

import (
	"context"
	"encoding/json"
	"net"

	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/test/bufconn"
)

const platformJSONCodecName = "platform-json"

type platformJSONCodec struct{}

func (platformJSONCodec) Marshal(v any) ([]byte, error)   { return json.Marshal(v) }
func (platformJSONCodec) Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func (platformJSONCodec) Name() string                    { return platformJSONCodecName }

func init() { encoding.RegisterCodec(platformJSONCodec{}) }

type IAMServiceClient interface {
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (LoginReply, error)
}

type SkillServiceClient interface {
	GetSkill(ctx context.Context, in *GetSkillRequest, opts ...grpc.CallOption) (Skill, error)
}

type iamServiceClient struct{ cc grpc.ClientConnInterface }

func NewIAMServiceClient(cc grpc.ClientConnInterface) IAMServiceClient {
	return iamServiceClient{cc: cc}
}
func (c iamServiceClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (LoginReply, error) {
	var out LoginReply
	err := c.cc.Invoke(ctx, "/aisphere.iam.v1.IAMService/Login", in, &out, opts...)
	return out, err
}

type skillServiceClient struct{ cc grpc.ClientConnInterface }

func NewSkillServiceClient(cc grpc.ClientConnInterface) SkillServiceClient {
	return skillServiceClient{cc: cc}
}
func (c skillServiceClient) GetSkill(ctx context.Context, in *GetSkillRequest, opts ...grpc.CallOption) (Skill, error) {
	var out Skill
	err := c.cc.Invoke(ctx, "/aisphere.skill.v1.SkillService/GetSkill", in, &out, opts...)
	return out, err
}

type iamGRPCServer interface {
	Login(context.Context, LoginRequest) (LoginReply, error)
}
type skillGRPCServer interface {
	GetSkill(context.Context, GetSkillRequest) (Skill, error)
}

func RegisterIAMServiceGRPCServer(s grpc.ServiceRegistrar, srv iamGRPCServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "aisphere.iam.v1.IAMService",
		HandlerType: (*iamGRPCServer)(nil),
		Methods: []grpc.MethodDesc{{MethodName: "Login", Handler: func(raw any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			var in LoginRequest
			if err := dec(&in); err != nil {
				return nil, err
			}
			if interceptor == nil {
				return raw.(iamGRPCServer).Login(ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: raw, FullMethod: "/aisphere.iam.v1.IAMService/Login"}
			handler := func(ctx context.Context, req any) (any, error) {
				return raw.(iamGRPCServer).Login(ctx, req.(LoginRequest))
			}
			return interceptor(ctx, in, info, handler)
		}}},
	}, srv)
}

type skillGRPCAdapter struct{ handler middleware.Handler }

func (a skillGRPCAdapter) GetSkill(ctx context.Context, req GetSkillRequest) (Skill, error) {
	ctx = gatewayx.GRPCServerContextFromIncomingMetadata(ctx, "/aisphere.skill.v1.SkillService/GetSkill")
	out, err := a.handler(ctx, req)
	if err != nil {
		return Skill{}, err
	}
	return out.(Skill), nil
}

func RegisterSkillServiceGRPCServer(s grpc.ServiceRegistrar, handler middleware.Handler) {
	srv := skillGRPCAdapter{handler: handler}
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "aisphere.skill.v1.SkillService",
		HandlerType: (*skillGRPCServer)(nil),
		Methods: []grpc.MethodDesc{{MethodName: "GetSkill", Handler: func(raw any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			var in GetSkillRequest
			if err := dec(&in); err != nil {
				return nil, err
			}
			if interceptor == nil {
				return raw.(skillGRPCServer).GetSkill(ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: raw, FullMethod: "/aisphere.skill.v1.SkillService/GetSkill"}
			handler := func(ctx context.Context, req any) (any, error) {
				return raw.(skillGRPCServer).GetSkill(ctx, req.(GetSkillRequest))
			}
			return interceptor(ctx, in, info, handler)
		}}},
	}, srv)
}

// RegisterPlatformGatewayGRPCInvokers is the generated shape used by Gateway.
// It uses generated gRPC clients, not in-memory service function calls.
func RegisterPlatformGatewayGRPCInvokers(registry *gatewayx.InvokerRegistry, iam IAMServiceClient, skill SkillServiceClient) error {
	opts := []grpc.CallOption{grpc.ForceCodec(platformJSONCodec{})}
	if err := registry.Register("/aisphere.iam.v1.IAMService/Login", gatewayx.GRPCUnaryInvoker(IAMServiceGatewayBindLogin, iam.Login, opts...)); err != nil {
		return err
	}
	return registry.Register("/aisphere.skill.v1.SkillService/GetSkill", gatewayx.GRPCUnaryInvoker(SkillServiceGatewayBindGetSkill, skill.GetSkill, opts...))
}

// newPlatformBufconn starts IAM and Skill services as real gRPC servers over
// bufconn. This validates the same client/server boundary as production without
// binding host ports during unit tests.
func newPlatformBufconn(ctx context.Context, iam iamService, skillHandler middleware.Handler) (*grpc.ClientConn, func(), error) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(grpc.ForceServerCodec(platformJSONCodec{}))
	RegisterIAMServiceGRPCServer(srv, iam)
	RegisterSkillServiceGRPCServer(srv, skillHandler)
	go func() { _ = srv.Serve(lis) }()
	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	cc, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(platformJSONCodec{})))
	if err != nil {
		srv.Stop()
		_ = lis.Close()
		return nil, nil, err
	}
	cc.Connect()
	cleanup := func() { _ = cc.Close(); srv.Stop(); _ = lis.Close() }
	_ = ctx
	return cc, cleanup, nil
}
