package grid

import (
	"errors"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestDirectionNamesAreTheWireNamesInOrder(t *testing.T) {
	if got, want := DirectionNames(), []string{"up", "down", "left", "right"}; !slices.Equal(got, want) {
		t.Fatalf("DirectionNames() = %v, want %v", got, want)
	}
}

func TestParseDirectionReadsEveryName(t *testing.T) {
	for _, d := range Directions() {
		got, err := ParseDirection(string(d))
		if err != nil || got != d {
			t.Errorf("ParseDirection(%q) = %q, %v; want %q", d, got, err, d)
		}
	}
}

func TestParseDirectionRejectsOtherText(t *testing.T) {
	for _, text := range []string{"", "Up", " up", "north", "u"} {
		_, err := ParseDirection(text)
		var illegal game.IllegalActionError
		if !errors.As(err, &illegal) {
			t.Errorf("ParseDirection(%q) error = %v, want an IllegalActionError", text, err)
		}
	}
}

func TestDirectionStepsOneCellAndOppositeStepsBack(t *testing.T) {
	from := Point{Row: 3, Col: 5}
	want := map[Direction]Point{Up: {2, 5}, Down: {4, 5}, Left: {3, 4}, Right: {3, 6}}
	for _, d := range Directions() {
		if got := d.From(from); got != want[d] {
			t.Errorf("%s.From(%v) = %v, want %v", d, from, got, want[d])
		}
		if back := d.Opposite().From(d.From(from)); back != from || d.Opposite() == d || d.Opposite().Opposite() != d {
			t.Errorf("%s.Opposite() = %s does not step back to %v", d, d.Opposite(), from)
		}
	}
}
