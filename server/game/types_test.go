package game_test

import (
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestCheckUserID(t *testing.T) {
	longest := game.UserID(strings.Repeat("é", game.MaxUserIDRunes))
	for _, tc := range []struct {
		name game.UserID
		ok   bool
	}{
		{"zoë", true},
		{"erin b", true},
		{longest, true},
		{"", false},
		{longest + "e", false},
		{" erin", false},
		{"erin ", false},
		{"erin\u00a0", false},
		{"ev\nil", false},
		{"bi\u202edi", false},
	} {
		if err := game.CheckUserID(tc.name); (err == nil) != tc.ok {
			t.Errorf("CheckUserID(%q) = %v, want ok %v", tc.name, err, tc.ok)
		}
	}
}
