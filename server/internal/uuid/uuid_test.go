package uuid_test

import (
	"regexp"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/internal/uuid"
)

var v4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewV4IsCanonicalAndRandom(t *testing.T) {
	seen := map[string]bool{}
	for range 64 {
		id := uuid.NewV4()
		if !v4.MatchString(id) || seen[id] {
			t.Fatalf("%q is not a fresh canonical v4 UUID", id)
		}
		seen[id] = true
	}
}
