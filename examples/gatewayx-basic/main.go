package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/restx"
	httpx "github.com/aisphereio/kernel/transportx/http"
)

func main() {
	store := gatewayx.NewMemorySessionStore()
	sid, _ := gatewayx.NewSessionID()
	_ = store.Set(context.Background(), gatewayx.Session{
		ID:        sid,
		Principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser, OrgID: "aisphere"},
		ExpiresAt: time.Now().Add(time.Hour),
	})

	conf := restx.Conf{
		Address: ":8080",
		Middlewares: restx.MiddlewaresConf{
			Recover:         true,
			SecurityHeaders: true,
			SanitizeHeaders: true,
			MaxBytes:        true,
			MaxBytesLimit:   8 << 20,
		},
	}
	srv := httpx.NewServer(restx.ServerOptions(conf)...)
	root := srv.Route("/")
	_ = gatewayx.RegisterServices(root, gatewayx.Service{
		Name:   "gateway-private",
		Prefix: "/api/v1",
		Filters: []httpx.FilterFunc{
			gatewayx.SessionMiddleware(gatewayx.SessionConfig{Store: store, Required: true}),
		},
		Routes: []gatewayx.Route{
			{Method: http.MethodGet, Path: "/me", Handler: func(ctx httpx.Context) error {
				p, _ := authn.PrincipalFromContext(ctx)
				return ctx.JSON(http.StatusOK, map[string]string{"subject_id": p.SubjectID, "org_id": p.OrgID})
			}},
		},
	})

	fmt.Println("example session cookie:", gatewayx.DefaultSessionCookie+"="+sid)
	_ = srv.Start(context.Background())
}
