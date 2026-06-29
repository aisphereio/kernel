package gatewayx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
)

// InternalClaims is the signed Gateway-to-service identity envelope.
type InternalClaims struct {
	Issuer          string          `json:"iss"`
	Audience        string          `json:"aud"`
	Subject         string          `json:"sub"`
	Actor           authn.Principal `json:"actor"`
	RequestID       string          `json:"request_id,omitempty"`
	AuthzDecisionID string          `json:"authz_decision_id,omitempty"`
	IssuedAt        int64           `json:"iat"`
	ExpiresAt       int64           `json:"exp"`
	JWTID           string          `json:"jti"`
}

// InternalJWT signs and verifies compact HS256 JWTs without adding a new public
// dependency. It is meant for Gateway -> internal service calls; external OIDC
// tokens should still be verified by the IdP/OIDC adapter.
type InternalJWT struct {
	issuer string
	secret []byte
	ttl    time.Duration
}

// NewInternalJWT creates a signer/verifier.
func NewInternalJWT(issuer string, secret []byte, ttl time.Duration) *InternalJWT {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &InternalJWT{issuer: issuer, secret: append([]byte(nil), secret...), ttl: ttl}
}

// Sign signs claims for audience. Subject defaults to issuer when empty.
func (j *InternalJWT) Sign(audience string, actor authn.Principal, requestID, decisionID string, now time.Time) (string, error) {
	if len(j.secret) == 0 {
		return "", errors.New("gatewayx: internal jwt secret is empty")
	}
	jti, err := randomID(16)
	if err != nil {
		return "", err
	}
	claims := InternalClaims{
		Issuer:          j.issuer,
		Audience:        audience,
		Subject:         j.issuer,
		Actor:           actor.Normalize(),
		RequestID:       requestID,
		AuthzDecisionID: decisionID,
		IssuedAt:        now.Unix(),
		ExpiresAt:       now.Add(j.ttl).Unix(),
		JWTID:           jti,
	}
	return j.signClaims(claims)
}

// Verify verifies a token and expected audience.
func (j *InternalJWT) Verify(token, audience string, now time.Time) (InternalClaims, error) {
	var zero InternalClaims
	if len(j.secret) == 0 {
		return zero, errors.New("gatewayx: internal jwt secret is empty")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return zero, errors.New("gatewayx: malformed jwt")
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(j.sign(signingInput)), []byte(parts[2])) {
		return zero, errors.New("gatewayx: invalid jwt signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, err
	}
	var claims InternalClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return zero, err
	}
	if claims.Issuer != j.issuer {
		return zero, fmt.Errorf("gatewayx: invalid issuer %q", claims.Issuer)
	}
	if audience != "" && claims.Audience != audience {
		return zero, fmt.Errorf("gatewayx: invalid audience %q", claims.Audience)
	}
	if claims.ExpiresAt <= now.Unix() {
		return zero, errors.New("gatewayx: jwt expired")
	}
	return claims, nil
}

func (j *InternalJWT) signClaims(claims InternalClaims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	head, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(head) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signingInput + "." + j.sign(signingInput), nil
}

func (j *InternalJWT) sign(input string) string {
	mac := hmac.New(sha256.New, j.secret)
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
