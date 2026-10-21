package grid

import (
	"regexp"
	"testing"
)

func TestCoordsPattern(t *testing.T) {
	tests := []struct {
		rows, cols int
		want       string
	}{
		{8, 8, `^\s*([a-hA-H])([1-8])\s*$`},
		{15, 15, `^\s*([a-oA-O])(1[0-5]|[1-9])\s*$`},
		{10, 1, `^\s*([a-aA-A])(1[0-0]|[1-9])\s*$`},
		{19, 26, `^\s*([a-zA-Z])(1[0-9]|[1-9])\s*$`},
	}
	for _, tc := range tests {
		if got := NewCoords(tc.rows, tc.cols).Pattern(); got != tc.want {
			t.Errorf("%dx%d: pattern %s, want %s", tc.rows, tc.cols, got, tc.want)
		}
	}
}

func TestNewCoordsRefusesBoardsItCannotName(t *testing.T) {
	for _, size := range [][2]int{{0, 8}, {20, 8}, {8, 0}, {8, 27}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%v: no panic", size)
				}
			}()
			NewCoords(size[0], size[1])
		}()
	}
}

func TestCoordsParse(t *testing.T) {
	gomoku := NewCoords(15, 15)
	tests := []struct {
		text   string
		want   Point
		wantOK bool
	}{
		{"a1", Point{Row: 0, Col: 0}, true},
		{"h8", Point{Row: 7, Col: 7}, true},
		{"o15", Point{Row: 14, Col: 14}, true},
		{"O15", Point{Row: 14, Col: 14}, true},
		{" d3\t", Point{Row: 2, Col: 3}, true},
		{"a10", Point{Row: 9, Col: 0}, true},
		{"", Point{}, false},
		{"p1", Point{}, false},
		{"a0", Point{}, false},
		{"a16", Point{}, false},
		{"a01", Point{}, false},
		{"8h", Point{}, false},
		{"h 8", Point{}, false},
		{"h8x", Point{}, false},
		{"h 8", Point{}, false},
	}
	matcher := regexp.MustCompile(gomoku.Pattern())
	for _, tc := range tests {
		got, ok := gomoku.Parse(tc.text)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("Parse(%q) = %+v, %t; want %+v, %t", tc.text, got, ok, tc.want, tc.wantOK)
		}
		if matcher.MatchString(tc.text) != tc.wantOK {
			t.Errorf("Pattern and Parse disagree on %q", tc.text)
		}
	}
}

func TestCoordsNameRoundTrips(t *testing.T) {
	reversi := NewCoords(8, 8)
	for row := range 8 {
		for col := range 8 {
			p := Point{Row: row, Col: col}
			if got, ok := reversi.Parse(reversi.Name(p)); !ok || got != p {
				t.Errorf("Parse(Name(%+v)) = %+v, %t", p, got, ok)
			}
		}
	}
	if got := reversi.Name(Point{Row: 2, Col: 3}); got != "d3" {
		t.Errorf("Name = %s, want d3", got)
	}
	if got := reversi.Range(); got != "a1 to h8" {
		t.Errorf("Range = %s, want a1 to h8", got)
	}
}
