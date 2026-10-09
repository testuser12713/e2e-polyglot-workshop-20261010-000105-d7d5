package store

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("geheim123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "geheim123" {
		t.Fatal("password was stored in clear")
	}
	if !VerifyPassword(hash, "geheim123") {
		t.Error("VerifyPassword rejected the correct password")
	}
	if VerifyPassword(hash, "falsch") {
		t.Error("VerifyPassword accepted a wrong password")
	}
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("HashPassword(\"\") should fail")
	}
}
