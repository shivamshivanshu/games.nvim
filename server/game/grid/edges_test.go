package grid

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func across(row, col int) Edge { return Edge{Dot: Point{Row: row, Col: col}, Orientation: EdgeAcross} }
func down(row, col int) Edge   { return Edge{Dot: Point{Row: row, Col: col}, Orientation: EdgeDown} }

func TestEdgesPattern(t *testing.T) {
	tests := []struct {
		rows, cols int
		want       string
	}{
		{1, 1, `^\s*(?:([aA])([1-2])[hH]|([a-bA-B])(1)[vV])\s*$`},
		{3, 3, `^\s*(?:([a-cA-C])([1-4])[hH]|([a-dA-D])([1-3])[vV])\s*$`},
		{9, 4, `^\s*(?:([a-dA-D])(10|[1-9])[hH]|([a-eA-E])([1-9])[vV])\s*$`},
	}
	for _, tc := range tests {
		if got := NewEdges(tc.rows, tc.cols).Pattern(); got != tc.want {
			t.Errorf("%dx%d: pattern %s, want %s", tc.rows, tc.cols, got, tc.want)
		}
	}
}

func TestNewEdgesRefusesBoardsItCannotName(t *testing.T) {
	for _, size := range [][2]int{{0, 3}, {3, 0}, {19, 3}, {3, 26}} {
		func() {
			defer func() {
				want := fmt.Sprintf("grid: no edges for %dx%d cells, want 1-18 rows and 1-25 columns", size[0], size[1])
				if got := recover(); got != want {
					t.Errorf("%v: panic %v, want %q", size, got, want)
				}
			}()
			NewEdges(size[0], size[1])
		}()
	}
}

func TestEdgesParse(t *testing.T) {
	tests := []struct {
		text   string
		want   Edge
		wantOK bool
	}{
		{"a1h", across(0, 0), true},
		{"c4h", across(3, 2), true},
		{"a1v", down(0, 0), true},
		{"d3v", down(2, 3), true},
		{"B2H", across(1, 1), true},
		{" b2V\t", down(1, 1), true},
		{"d1h", Edge{}, false},
		{"a4v", Edge{}, false},
		{"e1v", Edge{}, false},
		{"a5h", Edge{}, false},
		{"a0h", Edge{}, false},
		{"b2", Edge{}, false},
		{"b2x", Edge{}, false},
		{"b2hv", Edge{}, false},
		{"b 2h", Edge{}, false},
		{"h2b", Edge{}, false},
		{"", Edge{}, false},
	}
	edges := NewEdges(3, 3)
	for _, tc := range tests {
		got, ok := edges.Parse(tc.text)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("Parse(%q) = %+v, %t; want %+v, %t", tc.text, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestEdgesAllNamesEveryEdgeOnceAndRoundTrips(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {3, 3}, {2, 5}, {7, 7}, {12, 20}} {
		rows, cols := size[0], size[1]
		edges := NewEdges(rows, cols)
		all := edges.All()
		if want := (rows+1)*cols + rows*(cols+1); len(all) != want {
			t.Errorf("%dx%d: %d edges, want %d", rows, cols, len(all), want)
		}
		names := make(map[string]bool, len(all))
		for _, edge := range all {
			name := edges.Name(edge)
			if got, ok := edges.Parse(name); !ok || got != edge {
				t.Errorf("%dx%d: Parse(Name(%+v)) = %+v, %t", rows, cols, edge, got, ok)
			}
			if names[name] {
				t.Errorf("%dx%d: %s listed twice", rows, cols, name)
			}
			names[name] = true
		}
	}
	got := NewEdges(1, 1).All()
	if diff := cmp.Diff([]Edge{across(0, 0), down(0, 0), down(0, 1), across(1, 0)}, got); diff != "" {
		t.Errorf("1x1 order (-want +got):\n%s", diff)
	}
	if name := NewEdges(3, 3).Name(down(1, 3)); name != "d2v" {
		t.Errorf("Name = %s, want d2v", name)
	}
}

func TestEdgesContains(t *testing.T) {
	edges := NewEdges(3, 3)
	tests := []struct {
		edge Edge
		want bool
	}{
		{across(3, 2), true},
		{across(3, 3), false},
		{across(4, 0), false},
		{down(2, 3), true},
		{down(3, 0), false},
		{down(0, 4), false},
		{across(-1, 0), false},
		{Edge{Dot: Point{}, Orientation: "diagonal"}, false},
	}
	for _, tc := range tests {
		if got := edges.Contains(tc.edge); got != tc.want {
			t.Errorf("Contains(%+v) = %t, want %t", tc.edge, got, tc.want)
		}
	}
}

func TestEdgesBorderingAndSides(t *testing.T) {
	edges := NewEdges(3, 3)
	tests := []struct {
		edge Edge
		want []Point
	}{
		{across(0, 1), []Point{{Row: 0, Col: 1}}},
		{across(1, 1), []Point{{Row: 0, Col: 1}, {Row: 1, Col: 1}}},
		{across(3, 2), []Point{{Row: 2, Col: 2}}},
		{down(1, 0), []Point{{Row: 1, Col: 0}}},
		{down(1, 2), []Point{{Row: 1, Col: 1}, {Row: 1, Col: 2}}},
		{down(2, 3), []Point{{Row: 2, Col: 2}}},
	}
	for _, tc := range tests {
		got := edges.Bordering(tc.edge)
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("Bordering(%s) (-want +got):\n%s", edges.Name(tc.edge), diff)
		}
		for _, cell := range got {
			sides := Sides(cell)
			if len(sides) != 4 || !slices.Contains(sides, tc.edge) {
				t.Errorf("Sides(%+v) = %+v, missing %s", cell, sides, edges.Name(tc.edge))
			}
		}
	}
	want := []Edge{across(1, 2), across(2, 2), down(1, 2), down(1, 3)}
	if diff := cmp.Diff(want, Sides(Point{Row: 1, Col: 2})); diff != "" {
		t.Errorf("Sides (-want +got):\n%s", diff)
	}
}
