package password_test

import (
	"testing"

	"app/internal/pkg/password"
	"golang.org/x/crypto/bcrypt"
)

func TestVerifikasiLaravelPrefix(t *testing.T) {
	plain := "rahasia123"
	hash2a, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		t.Fatal(err)
	}
	laravel := "$2y$" + string(hash2a)[4:]

	if !password.Verifikasi(laravel, plain) {
		t.Fatal("hash $2y$ harus diterima")
	}
	if password.Verifikasi(laravel, "salah") {
		t.Fatal("password salah tidak boleh lolos")
	}
}

func TestHashRoundTrip(t *testing.T) {
	h, err := password.Hash("rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if !password.Verifikasi(h, "rahasia123") {
		t.Fatal("hash baru harus cocok")
	}
}
