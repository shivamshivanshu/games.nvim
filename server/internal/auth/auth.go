package auth

import (
	"crypto/sha256"
	"crypto/subtle"

	"golang.org/x/crypto/bcrypt"
)

// DefaultPasswordCost is the bcrypt cost new passwords are hashed at; tests
// pass bcrypt.MinCost, since each login checks a hash at the cost it was made.
const DefaultPasswordCost = bcrypt.DefaultCost

func HashPassword(pw string, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(pw), cost)
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
