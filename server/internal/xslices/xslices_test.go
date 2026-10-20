package xslices

import (
	"encoding/json"
	"testing"
)

func TestMapOfNilEncodesAsEmptyArray(t *testing.T) {
	raw, err := json.Marshal(Map([]int(nil), func(int) int { return 0 }))
	if err != nil || string(raw) != "[]" {
		t.Fatalf("got %s, %v; want []", raw, err)
	}
}

func TestMapValuesKeepsNilMapNil(t *testing.T) {
	if got := MapValues(map[string]int(nil), func(int) int { return 0 }); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
