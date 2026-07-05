//go:build ignore

// This demo shows the intended authn path:
//  1. Casdoor/OIDC signs a JWT with its private key.
//  2. Gateway loads issuer + JWKS and verifies the JWT locally.
//  3. Gateway strips spoofable X-Aisphere-* headers, injects trusted identity,
//     and dispatches to a backend gRPC-style operation.
//  4. The backend can either trust Gateway Principal or re-verify the same
//     Authorization bearer token with the same oidcx verifier.
//
// Run from the kernel repo with a Go version matching go.mod:
//
//	go run ./examples/authn_flow/demo.go
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/oidcx"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/golang-jwt/jwt/v4"
)

type demoInvoker struct{ backend authn.Authenticator }

func (i demoInvoker) Invoke(ctx context.Context, match gatewayx.RouteMatch, req gatewayx.DispatchRequest, target string) (gatewayx.DispatchResponse, error) {
	// Backend mode A: trust Gateway Principal propagated in context.
	edgePrincipal, _ := authn.PrincipalFromContext(ctx)

	// Backend mode B: defense-in-depth, re-verify the same bearer token.
	token := req.Headers["Authorization"]
	cred, _ := authn.BearerCredential(token)
	backendPrincipal, err := i.backend.Authenticate(ctx, cred)
	if err != nil {
		return gatewayx.DispatchResponse{Status: 401, Body: map[string]any{"error": err.Error()}}, nil
	}

	return gatewayx.DispatchResponse{Status: 200, Body: map[string]any{
		"gateway_subject": edgePrincipal.SubjectID,
		"backend_subject": backendPrincipal.SubjectID,
		"username":        backendPrincipal.Username,
		"owner":           backendPrincipal.OrgID,
		"groups":          backendPrincipal.Groups,
		"target":          target,
	}}, nil
}

func main() {
	issuer := "http://casdoor.demo"
	audience := "aisphere-demo"
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	kid := "demo-key-1"
	oidcServer := newOIDCServer(issuer, kid, &key.PublicKey)
	defer oidcServer.Close()

	cfg := oidcx.Config{
		Provider:      "casdoor",
		Issuer:        issuer,
		DiscoveryURL:  oidcServer.URL + "/.well-known/openid-configuration",
		Audience:      []string{audience},
		AllowedOwners: []string{"aisphere"},
		AllowedAlgs:   []string{"RS256"},
	}
	gatewayVerifier, _ := oidcx.New(cfg)
	backendVerifier, _ := oidcx.New(cfg)

	registry := gatewayx.NewMemoryRegistry()
	_ = registry.RegisterManifest(gatewayx.Manifest{Service: "demo", Namespace: "default", Routes: []gatewayx.GatewayRoute{{
		ID:       "demo.echo",
		Method:   http.MethodGet,
		Path:     "/v1/demo/echo",
		Upstream: gatewayx.UpstreamRef{Service: "demo", Namespace: "default", Operation: "demo.Echo"},
		Gateway:  gatewayx.GatewayPolicy{Exposure: accessv1.Exposure_AUTHENTICATED, AuthnMode: gatewayx.AuthnModeVerifyJWT},
	}}})

	dispatcher := gatewayx.NewDispatcher(registry, gatewayx.StaticHosts{"demo.default": "grpc://demo"}, demoInvoker{backend: backendVerifier}, gatewayx.WithAuthenticator(gatewayVerifier))
	token := signDemoJWT(key, kid, issuer, audience)

	resp, err := dispatcher.Dispatch(context.Background(), gatewayx.DispatchRequest{
		Method: http.MethodGet,
		Path:   "/v1/demo/echo",
		Headers: map[string]string{
			"Authorization":       "Bearer " + token,
			"X-Aisphere-Subject":  "attacker-spoof", // stripped by gateway before injection
			"X-Aisphere-Username": "mallory",
		},
	})
	if err != nil {
		panic(err)
	}
	b, _ := json.MarshalIndent(resp.Body, "", "  ")
	fmt.Println(string(b))
}

func newOIDCServer(issuer, kid string, pub *rsa.PublicKey) *httptest.Server {
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "jwks_uri": base + "/.well-known/jwks"})
		case "/.well-known/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{rsaJWK(kid, pub)}})
		default:
			http.NotFound(w, r)
		}
	}))
	base = srv.URL
	return srv
}

func rsaJWK(kid string, pub *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kid": kid,
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func signDemoJWT(key *rsa.PrivateKey, kid, issuer, audience string) string {
	claims := jwt.MapClaims{
		"iss":         issuer,
		"aud":         audience,
		"sub":         "user_123",
		"owner":       "aisphere",
		"name":        "alice",
		"displayName": "Alice",
		"email":       "alice@example.com",
		"groups":      []string{"engineering-platform", "sig-agent-runtime"},
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(30 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, _ := token.SignedString(key)
	return signed
}
