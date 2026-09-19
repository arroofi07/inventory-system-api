package password

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Verifikasi mencocokkan password dengan hash. Hash Laravel $2y$ dinormalkan ke $2a$.
func Verifikasi(hash, plain string) bool {
	if strings.HasPrefix(hash, "$2y$") {
		hash = "$2a$" + hash[4:]
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	return string(b), err
}
