package auth

import (
	"crypto/sha256"
	"crypto/subtle"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(pw string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
}

func CheckPassword(hash []byte, pw string) bool {
	return bcrypt.CompareHashAndPassword(hash, []byte(pw)) == nil
}

// PassphraseMatches compares digests in constant time, so neither the time
// taken nor the lengths given reveal anything of want.
func PassphraseMatches(want, got string) bool {
	wantSum, gotSum := sha256.Sum256([]byte(want)), sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(wantSum[:], gotSum[:]) == 1
}
