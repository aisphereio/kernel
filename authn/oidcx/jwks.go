package oidcx

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aisphereio/kernel/logx"
)

type keySet struct {
	cfg      Config
	mu       sync.RWMutex
	keys     map[string]any
	cachedAt time.Time
	jwksURL  string
	issuer   string
}

type discoveryDocument struct {
	Issuer  string `json:"issuer"`
	JWKSURL string `json:"jwks_uri"`
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyID   string `json:"kid"`
	KeyType string `json:"kty"`
	Use     string `json:"use"`
	Alg     string `json:"alg"`
	N       string `json:"n"`
	E       string `json:"e"`
	Crv     string `json:"crv"`
	X       string `json:"x"`
	Y       string `json:"y"`
}

func newKeySet(cfg Config) *keySet {
	return &keySet{cfg: cfg, keys: map[string]any{}, jwksURL: cfg.JWKSURL, issuer: cfg.Issuer}
}

func (s *keySet) issuerValue(ctx context.Context) (string, error) {
	s.mu.RLock()
	issuer := s.issuer
	s.mu.RUnlock()
	if issuer != "" {
		return issuer, nil
	}
	if _, err := s.ensure(ctx, false); err != nil {
		return "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issuer, nil
}

func (s *keySet) key(ctx context.Context, kid string) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	kid = strings.TrimSpace(kid)
	if key, ok := s.cachedKey(kid); ok {
		return key, nil
	}
	if _, err := s.ensure(ctx, true); err != nil {
		return nil, err
	}
	if key, ok := s.cachedKey(kid); ok {
		return key, nil
	}
	if kid == "" {
		return nil, fmt.Errorf("jwks has no default key")
	}
	return nil, fmt.Errorf("jwks key %q not found", kid)
}

func (s *keySet) cachedKey(kid string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.keys) == 0 || time.Since(s.cachedAt) > s.cfg.JWKSCacheTTL {
		return nil, false
	}
	if kid != "" {
		key, ok := s.keys[kid]
		return key, ok
	}
	if len(s.keys) == 1 {
		for _, key := range s.keys {
			return key, true
		}
	}
	return nil, false
}

func (s *keySet) ensure(ctx context.Context, force bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !force && len(s.keys) > 0 && time.Since(s.cachedAt) <= s.cfg.JWKSCacheTTL {
		return false, nil
	}
	jwksURL := s.jwksURL
	if jwksURL == "" || s.issuer == "" {
		doc, err := s.fetchDiscovery(ctx)
		if err != nil {
			return false, err
		}
		if jwksURL == "" {
			jwksURL = strings.TrimSpace(doc.JWKSURL)
			s.jwksURL = jwksURL
		}
		if s.issuer == "" {
			s.issuer = strings.TrimRight(strings.TrimSpace(doc.Issuer), "/")
		}
	}
	if jwksURL == "" {
		return false, fmt.Errorf("jwks_uri is empty")
	}
	doc, err := s.fetchJWKS(ctx, jwksURL)
	if err != nil {
		return false, err
	}
	keys := make(map[string]any, len(doc.Keys))
	for i, raw := range doc.Keys {
		key, err := raw.publicKey()
		if err != nil {
			if s.cfg.Logger != nil {
				s.cfg.Logger.Warn("oidc jwks key ignored", logx.String("kid", raw.KeyID), logx.Err(err))
			}
			continue
		}
		kid := strings.TrimSpace(raw.KeyID)
		if kid == "" {
			kid = fmt.Sprintf("__index_%d", i)
		}
		keys[kid] = key
	}
	if len(keys) == 0 {
		return false, fmt.Errorf("jwks has no usable signing keys")
	}
	s.keys = keys
	s.cachedAt = time.Now()
	return true, nil
}

func (s *keySet) fetchDiscovery(ctx context.Context) (discoveryDocument, error) {
	if s.cfg.DiscoveryURL == "" {
		return discoveryDocument{}, fmt.Errorf("discovery_url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.DiscoveryURL, nil)
	if err != nil {
		return discoveryDocument{}, err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return discoveryDocument{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discoveryDocument{}, fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}
	var doc discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return discoveryDocument{}, err
	}
	return doc, nil
}

func (s *keySet) fetchJWKS(ctx context.Context, url string) (jwksDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return jwksDocument{}, err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return jwksDocument{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return jwksDocument{}, fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}
	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return jwksDocument{}, err
	}
	return doc, nil
}

func (k jwk) publicKey() (any, error) {
	switch strings.ToUpper(strings.TrimSpace(k.KeyType)) {
	case "RSA":
		return k.rsaPublicKey()
	case "EC":
		return k.ecPublicKey()
	default:
		return nil, fmt.Errorf("unsupported jwk kty %q", k.KeyType)
	}
}

func (k jwk) rsaPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(k.N)
	if err != nil {
		return nil, fmt.Errorf("invalid rsa modulus: %w", err)
	}
	eBytes, err := decodeBase64URL(k.E)
	if err != nil {
		return nil, fmt.Errorf("invalid rsa exponent: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid rsa exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func (k jwk) ecPublicKey() (*ecdsa.PublicKey, error) {
	xBytes, err := decodeBase64URL(k.X)
	if err != nil {
		return nil, fmt.Errorf("invalid ec x: %w", err)
	}
	yBytes, err := decodeBase64URL(k.Y)
	if err != nil {
		return nil, fmt.Errorf("invalid ec y: %w", err)
	}
	var curve elliptic.Curve
	switch strings.ToUpper(strings.TrimSpace(k.Crv)) {
	case "P-256", "P256":
		curve = elliptic.P256()
	case "P-384", "P384":
		curve = elliptic.P384()
	case "P-521", "P521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported ec curve %q", k.Crv)
	}
	return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}, nil
}

func decodeBase64URL(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty base64url value")
	}
	return base64.RawURLEncoding.DecodeString(s)
}
