package grid

import "testing"

func TestCoordsPattern(t *testing.T) {
	tests := []struct {
		rows, cols int
		want       string
	}{
		{1, 1, `^\s*([aA])(1)\s*$`},
		{8, 8, `^\s*([a-hA-H])([1-8])\s*$`},
		{9, 9, `^\s*([a-iA-I])([1-9])\s*$`},
		{10, 2, `^\s*([a-bA-B])(10|[1-9])\s*$`},
		{15, 15, `^\s*([a-oA-O])(1[0-5]|[1-9])\s*$`},
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
	tests := []struct {
		rows, cols int
		text       string
		want       Point
		wantOK     bool
	}{
		{15, 15, "a1", Point{Row: 0, Col: 0}, true},
		{15, 15, "h8", Point{Row: 7, Col: 7}, true},
		{15, 15, "o15", Point{Row: 14, Col: 14}, true},
		{15, 15, "O15", Point{Row: 14, Col: 14}, true},
		{15, 15, " d3\t", Point{Row: 2, Col: 3}, true},
		{15, 15, "a10", Point{Row: 9, Col: 0}, true},
		{15, 15, "", Point{}, false},
		{15, 15, "p1", Point{}, false},
		{15, 15, "a0", Point{}, false},
		{15, 15, "a16", Point{}, false},
		{15, 15, "a01", Point{}, false},
		{15, 15, "8h", Point{}, false},
		{15, 15, "h 8", Point{}, false},
		{15, 15, "h8x", Point{}, false},
		{15, 15, "h\u00a08", Point{}, false},
		{9, 9, "i9", Point{Row: 8, Col: 8}, true},
		{9, 9, "a10", Point{}, false},
		{9, 9, "j1", Point{}, false},
		{10, 2, "b10", Point{Row: 9, Col: 1}, true},
		{10, 2, "a11", Point{}, false},
		{19, 19, "s19", Point{Row: 18, Col: 18}, true},
		{19, 19, "t1", Point{}, false},
		{19, 19, "a20", Point{}, false},
		{19, 26, "z19", Point{Row: 18, Col: 25}, true},
		{19, 26, "Z1", Point{Row: 0, Col: 25}, true},
		{1, 1, "a1", Point{}, true},
		{1, 1, "b1", Point{}, false},
		{1, 1, "a2", Point{}, false},
	}
	for _, tc := range tests {
		got, ok := NewCoords(tc.rows, tc.cols).Parse(tc.text)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("%dx%d Parse(%q) = %+v, %t; want %+v, %t", tc.rows, tc.cols, tc.text, got, ok, tc.want, tc.wantOK)
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
