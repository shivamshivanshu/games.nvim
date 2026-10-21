package grid

import (
	"encoding/json"
	"testing"
)

func TestCellLinesWire(t *testing.T) {
	tests := []struct {
		name  string
		lines [][]Point
		want  string
	}{
		{"none", nil, `[]`},
		{"an empty line", [][]Point{nil}, `[[]]`},
		{"1-based", [][]Point{{{Row: 0, Col: 0}, {Row: 2, Col: 4}}}, `[[{"row":1,"col":1},{"row":3,"col":5}]]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(CellLines(tc.lines))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
