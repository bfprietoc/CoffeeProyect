package service

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store/memory"
	"errors"
	"strings"
	"testing"
)

func TestRegisterOrLoginWithGoogle_NewUser(t *testing.T) {
	svc := NewUserService(memory.NewUserStore(), "test-secret")

	tokens, user, err := svc.RegisterOrLoginWithGoogle("  Juan.Perez@Example.com ", "Juan Pérez")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "juan.perez@example.com" {
		t.Fatalf("email not normalized: %q", user.Email)
	}
	if user.Name != "Juan Pérez" {
		t.Fatalf("unexpected name: %q", user.Name)
	}
	if user.PasswordHash != "" {
		t.Fatal("google-created user must have empty password hash")
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected token pair")
	}

	// el token de acceso debe ser válido para el middleware
	if _, err := auth.ValidateAccessToken(tokens.AccessToken, "test-secret"); err != nil {
		t.Fatalf("access token invalid: %v", err)
	}
}

func TestRegisterOrLoginWithGoogle_ExistingUser(t *testing.T) {
	store := memory.NewUserStore()
	svc := NewUserService(store, "test-secret")

	// usuario creado antes con contraseña
	if _, _, err := svc.Register("Ana", "ana@example.com", "supersecret1"); err != nil {
		t.Fatal(err)
	}

	tokens, user, err := svc.RegisterOrLoginWithGoogle("ana@example.com", "Ana G.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "ana@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.PasswordHash == "" {
		t.Fatal("existing user must keep its password hash")
	}
	if tokens.AccessToken == "" {
		t.Fatal("expected token pair")
	}

	// sigue pudiendo login con contraseña (mismo usuario, no se rompió)
	if _, _, err := svc.Login("ana@example.com", "supersecret1"); err != nil {
		t.Fatalf("password login broken after google login: %v", err)
	}
}

func TestRegisterOrLoginWithGoogle_NameFallback(t *testing.T) {
	svc := NewUserService(memory.NewUserStore(), "test-secret")

	_, user, err := svc.RegisterOrLoginWithGoogle("maria.gomez@example.com", "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "Maria.gomez" {
		t.Fatalf("expected name derived from email, got %q", user.Name)
	}
}

func TestRegisterOrLoginWithGoogle_EmptyEmail(t *testing.T) {
	svc := NewUserService(memory.NewUserStore(), "test-secret")

	_, _, err := svc.RegisterOrLoginWithGoogle("", "Sin Email")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestDisplayNameFromEmail(t *testing.T) {
	cases := map[string]string{
		"juan.perez@example.com": "Juan.perez",
		"a@b.com":                "A",
		"no-at-sign":             "No-at-sign",
	}
	for in, want := range cases {
		if got := displayNameFromEmail(in); got != want {
			t.Errorf("displayNameFromEmail(%q) = %q, want %q", in, got, want)
		}
	}
	if !strings.HasPrefix(displayNameFromEmail("x@y.com"), "X") {
		t.Fatal("expected capitalized first letter")
	}
}
