// Package credential provides shared password hashing helpers for the user
// repository adapters. Both the Postgres and in-memory adapters hash with
// bcrypt so credential verification behaves identically across backends.
package credential

import (
	"golang.org/x/crypto/bcrypt"
)

// cost is the bcrypt work factor. 12 gives a good balance of security and
// acceptable login latency for a demo/portfolio workload.
const cost = 12

// CompareHashAndPassword reports whether the stored bcrypt hash matches the
// supplied plaintext password.
func CompareHashAndPassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

// HashPassword returns a bcrypt digest of the password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// DummyHash is a valid bcrypt digest of a throwaway string. Repositories
// run a comparison against it when the supplied username does not exist, so
// the server spends roughly the same time regardless of whether the account
// is known. This keeps user enumeration (and timing side-channels) harder.
const DummyHash = "$2a$12$5tnU/h/e828p1N1HwtC1/..KICej6tmU0RBmyL6m6bC.g9/lhTYJy"
