//go:build ignore

// This demo is intentionally excluded from normal `go test ./...` because it
// depends on generated files. Run `make api` first, then copy this pattern into
// a real service package.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/contextx"
	skillv1 "github.com/aisphereio/kernel/examples/secure-skill-service/api/skill/v1"
	"github.com/aisphereio/kernel/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type skillService struct {
	skillv1.UnimplementedSkillServiceServer
}

func (skillService) DownloadSkillPackage(ctx context.Context, req *skillv1.DownloadSkillPackageRequest) (*skillv1.DownloadSkillPackageReply, error) {
	return &skillv1.DownloadSkillPackageReply{PackageData: []byte("package:" + req.SkillId)}, nil
}

func (skillService) DeleteSkill(ctx context.Context, req *skillv1.DeleteSkillRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

type demoGuard struct{}

func (demoGuard) Require(ctx context.Context, req authz.CheckRequest) (authz.Decision, error) {
	if req.Subject.IsZero() || req.Resource.IsZero() || strings.TrimSpace(req.Permission) == "" {
		return authz.Deny("missing authz input"), authz.ErrPermissionDenied("missing authz input")
	}
	return authz.Decision{Allowed: true, Effect: authz.DecisionAllow, Reason: "demo allow", ConsistencyToken: "dec_demo_123"}, nil
}

func (demoGuard) RequireScopedToken(ctx context.Context, req authz.ScopedTokenRequest) (string, authz.Decision, error) {
	decision, err := demoGuard{}.Require(ctx, authz.CheckRequest{Subject: req.Subject, Resource: req.Resource, Permission: req.Action, TenantID: req.TenantID})
	if err != nil {
		return "", decision, err
	}
	// A real IAM implementation signs a short-lived JWT with aud/scope/actor/decision.
	return "demo-scoped-token", decision, nil
}

type demoVerifier struct{}

func (demoVerifier) VerifyToken(ctx context.Context, token string) (grpcx.VerifiedToken, error) {
	if token == "" {
		return grpcx.VerifiedToken{}, errors.New("empty token")
	}
	return grpcx.VerifiedToken{
		Kind:        grpcx.TokenKindScoped,
		SubjectID:   "u_demo",
		SubjectType: "user",
		TenantID:    "t_demo",
		DecisionID:  "dec_demo_123",
		Scopes:      []string{"skill:demo:download"},
		AuthMethod:  "demo",
	}, nil
}

func main() {
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req_demo_123")
	ctx = contextx.WithTraceID(ctx, "trace_demo_123")
	ctx = contextx.WithPrincipal(ctx, &contextx.Principal{SubjectID: "u_demo", TenantID: "t_demo", AuthMethod: "demo"})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	guard := demoGuard{}
	srv := grpc.NewServer(grpcx.ServerOptions(grpcx.ServerConfig{
		ServiceName:      "skill-service",
		EnableContext:    true,
		EnableValidation: true,
		EnableRecovery:   true,
		EnableTokenAuth:  true,
		TokenAuth: grpcx.TokenAuthConfig{
			Required: true,
			Verifier: demoVerifier{},
		},
		ExtraUnary: []grpc.UnaryServerInterceptor{
			skillv1.SkillServiceAuthzUnaryServerInterceptor(guard),
		},
	})...)
	skillv1.RegisterSkillServiceServer(srv, skillService{})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), append(grpcx.DialOptions(grpcx.ClientConfig{ServiceName: "workflow-service", EnableContext: true}), grpc.WithTransportCredentials(insecure.NewCredentials()))...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	raw := skillv1.NewSkillServiceClient(conn)
	secure := skillv1.NewSkillServiceSecureClient(raw, guard)

	reply, err := secure.DownloadSkillPackage(ctx, &skillv1.DownloadSkillPackageRequest{SkillId: "demo"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("downloaded %q\n", string(reply.PackageData))

	_, err = secure.DeleteSkill(ctx, &skillv1.DeleteSkillRequest{SkillId: "demo"})
	if err != nil {
		panic(err)
	}
	fmt.Println("deleted demo skill")
}
