package casdoor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aisphereio/kernel/authn"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/golang-jwt/jwt/v4"
)

func TestExchangeCodeSendsRedirectURIAndCodeVerifier(t *testing.T) {
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login/oauth/access_token" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"test"}`))
	}))
	defer server.Close()

	client := &Client{cfg: Config{
		Endpoint:     server.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		HTTPClient:   server.Client(),
	}}
	_, _, err := client.ExchangeCode(context.Background(), authn.AuthCodeExchangeRequest{
		Code:         "code-1",
		RedirectURI:  "http://localhost:3000/auth/callback",
		CodeVerifier: "verifier-1",
	})
	if err == nil {
		t.Fatal("expected exchange error")
	}
	if got.Get("grant_type") != "authorization_code" {
		t.Fatalf("grant_type = %q", got.Get("grant_type"))
	}
	if got.Get("code") != "code-1" {
		t.Fatalf("code = %q", got.Get("code"))
	}
	if got.Get("redirect_uri") != "http://localhost:3000/auth/callback" {
		t.Fatalf("redirect_uri = %q", got.Get("redirect_uri"))
	}
	if got.Get("code_verifier") != "verifier-1" {
		t.Fatalf("code_verifier = %q", got.Get("code_verifier"))
	}
	if got.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q", got.Get("client_id"))
	}
	if got.Get("client_secret") != "client-secret" {
		t.Fatalf("client_secret = %q", got.Get("client_secret"))
	}
}

func TestVerifyTokenAllowsSmallCasdoorClockSkew(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	certificate := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey}))

	now := time.Now()
	claims := casdoorsdk.Claims{
		User: casdoorsdk.User{
			Owner:       "aisphere",
			Name:        "user",
			Id:          "user-id",
			DisplayName: "User",
			Email:       "user@example.local",
		},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "http://casdoor.local",
			Subject:   "user-id",
			Audience:  jwt.ClaimStrings{"client-id"},
			IssuedAt:  jwt.NewNumericDate(now.Add(10 * time.Second)),
			NotBefore: jwt.NewNumericDate(now.Add(10 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		TokenType: "access-token",
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	client := &Client{cfg: Config{
		Endpoint:         "http://casdoor.local",
		ClientID:         "client-id",
		ClientSecret:     "client-secret",
		JWTCertificate:   certificate,
		Certificate:      certificate,
		OrganizationName: "aisphere",
		ApplicationName:  "aisphere",
	}}
	principal, err := client.VerifyToken(context.Background(), authn.VerifyTokenRequest{Token: token})
	if err != nil {
		t.Fatalf("VerifyToken with default leeway returned error: %v", err)
	}
	if principal.SubjectID != "user-id" || principal.Username != "user" {
		t.Fatalf("principal = %+v", principal)
	}

	_, err = client.VerifyToken(context.Background(), authn.VerifyTokenRequest{Token: token, Leeway: time.Nanosecond})
	if err == nil {
		t.Fatal("expected tiny leeway to reject a token that is not valid yet")
	}
}
