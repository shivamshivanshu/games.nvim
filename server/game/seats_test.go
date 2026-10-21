package game

import "testing"

func TestNextSeat(t *testing.T) {
	cases := []struct {
		name string
		in   []bool
		from int
		want int
	}{
		{"the next seat", []bool{true, true, true, true}, 1, 2},
		{"wraps round", []bool{true, true, true, true}, 3, 0},
		{"skips seats out", []bool{true, false, false, true}, 0, 3},
		{"skips past the end", []bool{false, true, false, false}, 2, 1},
		{"from itself last", []bool{false, true, false}, 1, 1},
		{"from when none is in", []bool{false, false, false}, 2, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NextSeat(len(c.in), c.from, func(i int) bool { return c.in[i] }); got != c.want {
				t.Errorf("NextSeat(%d, %d) over %v = %d, want %d", len(c.in), c.from, c.in, got, c.want)
			}
		})
	}
}
