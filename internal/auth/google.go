package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// defaultGoogleJWKSURL is where Google serves the public keys that sign ID
// tokens. Overridable in tests via verifyGoogleIDTokenWithURL.
const defaultGoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

// GoogleIdentity are the claims we use from a verified Google ID token.
type GoogleIdentity struct {
	Email string
	Name  string
}

// VerifyGoogleIDToken validates a Google ID token (JWT) and returns the
// identity claims. It checks:
//   - RS256 signature against Google's JWKS public keys
//   - issuer (accounts.google.com) and expiry (jwt/v5 registered claims)
//   - audience: the token must have been issued for `audience`
//     (our OAuth Client ID)
//   - a non-empty email claim
//
// No client secret is needed: ID tokens are verified with Google's public
// keys.
func VerifyGoogleIDToken(ctx context.Context, idToken, audience string) (GoogleIdentity, error) {
	return verifyGoogleIDTokenWithURL(ctx, idToken, audience, defaultGoogleJWKSURL)
}

type googleClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

func verifyGoogleIDTokenWithURL(ctx context.Context, idToken, audience, jwksURL string) (GoogleIdentity, error) {
	token, err := jwt.ParseWithClaims(idToken, &googleClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return jwksKey(ctx, jwksURL, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(audience),
	)
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("verify google id token: %w", err)
	}

	claims, ok := token.Claims.(*googleClaims)
	if !ok || !token.Valid {
		return GoogleIdentity{}, fmt.Errorf("verify google id token: invalid claims")
	}
	// Google firma con ambos issuers según el contexto; aceptar los dos.
	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return GoogleIdentity{}, fmt.Errorf("verify google id token: unexpected issuer %q", claims.Issuer)
	}
	if claims.Email == "" {
		return GoogleIdentity{}, fmt.Errorf("verify google id token: no email claim")
	}
	return GoogleIdentity{Email: claims.Email, Name: claims.Name}, nil
}

// ---- JWKS fetching with a short TTL cache (keys rotate rarely) ----

type jwkKey struct {
	KID string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"` // modulus, base64url
	E   string `json:"e"` // exponent, base64url
}

type jwksDoc struct {
	Keys []jwkKey `json:"keys"`
}

var (
	jwksCacheMu    sync.Mutex
	jwksCache      = map[string]*rsa.PublicKey{} // jwksURL+"|"+kid → key
	jwksFetchedAt  = map[string]time.Time{}      // jwksURL → last fetch
	jwksCacheTTL   = 10 * time.Minute
	jwksHTTPClient = &http.Client{Timeout: 5 * time.Second}
)

func jwksKey(ctx context.Context, jwksURL, kid string) (*rsa.PublicKey, error) {
	jwksCacheMu.Lock()
	if key, ok := jwksCache[jwksURL+"|"+kid]; ok {
		jwksCacheMu.Unlock()
		return key, nil
	}
	fresh := time.Since(jwksFetchedAt[jwksURL]) < jwksCacheTTL
	jwksCacheMu.Unlock()

	if fresh {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := jwksHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read jwks: %w", err)
	}

	var doc jwksDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}

	jwksCacheMu.Lock()
	for _, k := range doc.Keys {
		key, err := parseJWK(k)
		if err != nil {
			continue
		}
		jwksCache[jwksURL+"|"+k.KID] = key
	}
	jwksFetchedAt[jwksURL] = time.Now()
	key, ok := jwksCache[jwksURL+"|"+kid]
	jwksCacheMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	return key, nil
}

func parseJWK(k jwkKey) (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}
