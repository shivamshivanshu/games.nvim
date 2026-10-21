package game_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestNumberSeatsSeatsUsersInOrder(t *testing.T) {
	order, seats := game.NumberSeats([]game.UserID{"carol", "alice", "bob"})
	if want := []game.Seat{"p1", "p2", "p3"}; !cmp.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
	if want := map[game.UserID]game.Seat{"carol": "p1", "alice": "p2", "bob": "p3"}; !cmp.Equal(seats, want) {
		t.Errorf("seats = %v, want %v", seats, want)
	}
}
