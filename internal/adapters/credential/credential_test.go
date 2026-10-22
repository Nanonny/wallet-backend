package credential

import (
	"testing"
)

func TestHashAndComparePassword(t *testing.T) {
	hash, err := HashPassword("abc12345")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CompareHashAndPassword(hash, "abc12345") {
		t.Fatal("correct password must verify")
	}
	if CompareHashAndPassword(hash, "wrong9999") {
		t.Fatal("wrong password must not verify")
	}
}

func TestDummyHashComparesCleanly(t *testing.T) {
	// The dummy hash must be a structurally valid bcrypt digest so that
	// comparisons cost the same as real ones.
	if CompareHashAndPassword(DummyHash, "any-password-1") {
		t.Fatal("dummy hash must not match arbitrary passwords")
	}
}
