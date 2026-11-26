package memory

import (
	"fmt"
	"testing"

	"coffeeproyect/internal/domain"
)

// Regresión: randomHex devolvía una cadena fija ("01234567"), así todos
// los usuarios y direcciones compartían ID — filtrando datos entre usuarios.
// Cada usuario y cada dirección debe recibir un ID único.
func TestUserAndAddressIDsAreUnique(t *testing.T) {
	s := NewUserStore()

	u1, err := s.Create(domain.User{Email: "uni-1@test.com", Name: "Uni 1"})
	if err != nil {
		t.Fatalf("create user 1: %v", err)
	}
	u2, err := s.Create(domain.User{Email: "uni-2@test.com", Name: "Uni 2"})
	if err != nil {
		t.Fatalf("create user 2: %v", err)
	}
	if u1.ID == u2.ID {
		t.Fatalf("users share the same ID: %s", u1.ID)
	}

	a1, err := s.AddAddress(domain.Address{UserID: u1.ID, Label: "Casa", Street: "X", City: "Y", Department: "Z"})
	if err != nil {
		t.Fatalf("add address 1: %v", err)
	}
	a2, err := s.AddAddress(domain.Address{UserID: u2.ID, Label: "Otra", Street: "X", City: "Y", Department: "Z"})
	if err != nil {
		t.Fatalf("add address 2: %v", err)
	}
	if a1.ID == a2.ID {
		t.Fatalf("addresses share the same ID: %s", a1.ID)
	}

	// aislamiento por usuario: cada uno ve sus direcciones
	list1, err := s.GetAddresses(u1.ID)
	if err != nil {
		t.Fatalf("get addresses 1: %v", err)
	}
	list2, err := s.GetAddresses(u2.ID)
	if err != nil {
		t.Fatalf("get addresses 2: %v", err)
	}
	if len(list1) != 1 || list1[0].Label != "Casa" {
		t.Fatalf("user 1 sees %v, want [Casa]", list1)
	}
	if len(list2) != 1 || list2[0].Label != "Otra" {
		t.Fatalf("user 2 sees %v, want [Otra]", list2)
	}
}

func TestSequentialUserIDs(t *testing.T) {
	s := NewUserStore()
	ids := map[string]bool{}
	for i := 0; i < 5; i++ {
		u, err := s.Create(domain.User{Email: fmt.Sprintf("seq-%d@test.com", i), Name: "Seq"})
		if err != nil {
			t.Fatalf("create user %d: %v", i, err)
		}
		if ids[u.ID] {
			t.Fatalf("duplicate user ID: %s", u.ID)
		}
		ids[u.ID] = true
	}
}
