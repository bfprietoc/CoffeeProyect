package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeJWKSServer serves a JWKS document with a single RSA key (kid "test-kid").
func fakeJWKSServer(t *testing.T, key *rsa.PublicKey) *httptest.Server {
	t.Helper()
	b64 := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	doc := fmt.Sprintf(`{"keys":[{"kid":"test-kid","kty":"RSA","n":"%s","e":"%s"}]}`,
		b64(key.N.Bytes()), b64([]byte{byte(key.E >> 16), byte(key.E >> 8), byte(key.E)}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, doc)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testClaims(email, name, aud, iss string, exp time.Time) *jwt.RegisteredClaims {
	return &jwt.RegisteredClaims{
		Issuer:    iss,
		Audience:  jwt.ClaimStrings{aud},
		Subject:   email,
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
}

func TestVerifyGoogleIDToken_Valid(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := fakeJWKSServer(t, &key.PublicKey)

	type claimsWithEmail struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		jwt.RegisteredClaims
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &claimsWithEmail{
		Email: "juan@example.com",
		Name:  "Juan Pérez",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "accounts.google.com",
			Audience:  jwt.ClaimStrings{"my-client-id"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	token.Header["kid"] = "test-kid"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	id, err := verifyGoogleIDTokenWithURL(context.Background(), signed, "my-client-id", srv.URL)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if id.Email != "juan@example.com" || id.Name != "Juan Pérez" {
		t.Fatalf("unexpected identity: %+v", id)
	}
}

func TestVerifyGoogleIDToken_WrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := fakeJWKSServer(t, &key.PublicKey)

	type ce struct {
		jwt.RegisteredClaims
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &ce{RegisteredClaims: *testClaims("a@b.com", "", "other-client", "accounts.google.com", time.Now().Add(time.Hour))})
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(key)

	if _, err := verifyGoogleIDTokenWithURL(context.Background(), signed, "my-client-id", srv.URL); err == nil {
		t.Fatal("expected audience error")
	}
}

func TestVerifyGoogleIDToken_WrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := fakeJWKSServer(t, &key.PublicKey)

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}{Email: "a@b.com", RegisteredClaims: *testClaims("a@b.com", "", "my-client-id", "evil.example.com", time.Now().Add(time.Hour))})
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(key)

	if _, err := verifyGoogleIDTokenWithURL(context.Background(), signed, "my-client-id", srv.URL); err == nil {
		t.Fatal("expected issuer error")
	}
}

func TestVerifyGoogleIDToken_Expired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := fakeJWKSServer(t, &key.PublicKey)

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}{Email: "a@b.com", RegisteredClaims: *testClaims("a@b.com", "", "my-client-id", "accounts.google.com", time.Now().Add(-time.Hour))})
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(key)

	if _, err := verifyGoogleIDTokenWithURL(context.Background(), signed, "my-client-id", srv.URL); err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestVerifyGoogleIDToken_UnknownKid(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := fakeJWKSServer(t, &key.PublicKey)

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}{Email: "a@b.com", RegisteredClaims: *testClaims("a@b.com", "", "my-client-id", "accounts.google.com", time.Now().Add(time.Hour))})
	tok.Header["kid"] = "not-in-jwks"
	signed, _ := tok.SignedString(key)

	if _, err := verifyGoogleIDTokenWithURL(context.Background(), signed, "my-client-id", srv.URL); err == nil {
		t.Fatal("expected unknown kid error")
	}
}

func TestVerifyGoogleIDToken_MissingEmail(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := fakeJWKSServer(t, &key.PublicKey)

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &struct {
		jwt.RegisteredClaims
	}{RegisteredClaims: *testClaims("", "", "my-client-id", "accounts.google.com", time.Now().Add(time.Hour))})
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(key)

	if _, err := verifyGoogleIDTokenWithURL(context.Background(), signed, "my-client-id", srv.URL); err == nil {
		t.Fatal("expected missing email error")
	}
}
