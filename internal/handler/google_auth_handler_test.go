package handler

import (
	"coffeeproyect/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type googleResponse struct {
	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"tokens"`
}

func newGoogleTestMux(verifyErr error, identity auth.GoogleIdentity) (*http.ServeMux, *googleResponse) {
	out := &googleResponse{}
	svc := newTestUserService()
	h := NewAuthHandler(svc, nil).
		WithGoogleClientID("test-client-id").
		WithGoogleVerifier(func(_ context.Context, _, _ string) (auth.GoogleIdentity, error) {
			if verifyErr != nil {
				return auth.GoogleIdentity{}, verifyErr
			}
			return identity, nil
		})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/google", h.GoogleLogin)
	mux.HandleFunc("POST /auth/register", h.Register)
	return mux, out
}

func TestGoogleLogin_DisabledWithoutClientID(t *testing.T) {
	svc := newTestUserService()
	h := NewAuthHandler(svc, nil) // sin WithGoogleClientID
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/google", h.GoogleLogin)

	rec := post(mux, "/auth/google", map[string]string{"credential": "x"}, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOOGLE_AUTH_DISABLED") {
		t.Fatalf("expected GOOGLE_AUTH_DISABLED code, got %s", rec.Body.String())
	}
}

func TestGoogleLogin_EmptyCredential(t *testing.T) {
	mux, _ := newGoogleTestMux(nil, auth.GoogleIdentity{})
	rec := post(mux, "/auth/google", map[string]string{"credential": ""}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGoogleLogin_InvalidToken(t *testing.T) {
	mux, _ := newGoogleTestMux(errors.New("bad token"), auth.GoogleIdentity{})
	rec := post(mux, "/auth/google", map[string]string{"credential": "forged"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INVALID_GOOGLE_TOKEN") {
		t.Fatalf("expected INVALID_GOOGLE_TOKEN code, got %s", rec.Body.String())
	}
}

func TestGoogleLogin_NewUser(t *testing.T) {
	mux, _ := newGoogleTestMux(nil, auth.GoogleIdentity{Email: "nuevo@example.com", Name: "Nuevo Usuario"})

	rec := post(mux, "/auth/google", map[string]string{"credential": "good-token"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var out googleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.User.Email != "nuevo@example.com" || out.User.ID == "" {
		t.Fatalf("unexpected user: %+v", out.User)
	}
	if out.Tokens.AccessToken == "" || out.Tokens.RefreshToken == "" {
		t.Fatal("expected token pair")
	}

	// el access token emitido debe ser válido para el middleware de auth
	if _, err := auth.ValidateAccessToken(out.Tokens.AccessToken, testSecret); err != nil {
		t.Fatalf("access token invalid: %v", err)
	}
}

func TestGoogleLogin_ExistingUserSameAccount(t *testing.T) {
	mux, _ := newGoogleTestMux(nil, auth.GoogleIdentity{Email: "ana@example.com", Name: "Ana"})

	// registro previo por email/contraseña
	rec := post(mux, "/auth/register", map[string]string{
		"name": "Ana", "email": "ana@example.com", "password": "supersecret1",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", rec.Code, rec.Body.String())
	}
	var reg struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}

	// login con Google del mismo email → mismo usuario
	rec = post(mux, "/auth/google", map[string]string{"credential": "good-token"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("google login failed: %d %s", rec.Code, rec.Body.String())
	}
	var out googleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.User.ID != reg.User.ID {
		t.Fatalf("google login created a different user: %q != %q", out.User.ID, reg.User.ID)
	}
}
