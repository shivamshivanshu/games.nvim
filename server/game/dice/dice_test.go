package dice

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 0)) }

func TestRollIsFixedBySeed(t *testing.T) {
	if a, b := Roll(seeded(4), 10, Six), Roll(seeded(4), 10, Six); !cmp.Equal(a, b) {
		t.Errorf("same seed rolled %v then %v", a, b)
	}
	if a, b := Roll(seeded(4), 10, Six), Roll(seeded(5), 10, Six); cmp.Equal(a, b) {
		t.Errorf("seeds 4 and 5 both rolled %v", a)
	}
}

func TestRollDrawsOncePerDieInOrder(t *testing.T) {
	rng := seeded(9)
	split := append(Roll(rng, 2, Six), Roll(rng, 3, Six)...)
	if whole := Roll(seeded(9), 5, Six); !cmp.Equal(split, whole) {
		t.Errorf("2 then 3 dice rolled %v, 5 dice rolled %v", split, whole)
	}
}

func TestRollShowsEveryFaceAndNoOther(t *testing.T) {
	for _, sides := range []int{2, Six, 20} {
		faces := Roll(seeded(1), 200*sides, sides)
		for face := Face(1); face <= Face(sides); face++ {
			if !slices.Contains(faces, face) {
				t.Errorf("d%d never rolled %d", sides, face)
			}
		}
		if bad := slices.IndexFunc(faces, func(f Face) bool { return f < 1 || f > Face(sides) }); bad >= 0 {
			t.Errorf("d%d rolled %d", sides, faces[bad])
		}
	}
}

func TestRollOfNoDiceIsEmpty(t *testing.T) {
	if faces := Roll(seeded(1), 0, Six); faces == nil || len(faces) != 0 {
		t.Errorf("Roll of 0 dice = %#v, want an empty non-nil slice", faces)
	}
}

func TestDieDrawsWhatRollingOneDieDraws(t *testing.T) {
	die, roll := seeded(3), seeded(3)
	for range 20 {
		if a, b := Die(die, Six), Roll(roll, 1, Six)[0]; a != b {
			t.Fatalf("Die rolled %d, Roll of one die %d", a, b)
		}
	}
}
