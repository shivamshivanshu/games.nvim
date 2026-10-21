// Package dice rolls dice from the rng the core gives a step, so a replay of
// the same steps rolls the same faces.
package dice

import "math/rand/v2"

// Face is the number a die shows, 1 to its sides.
type Face int

// Six is the sides of the common die.
const Six = 6

// Roll throws n dice of sides faces each, one draw from rng per die in order,
// so rolling a then b dice draws exactly what rolling a+b does.
func Roll(rng *rand.Rand, n, sides int) []Face {
	faces := make([]Face, n)
	for i := range faces {
		faces[i] = Die(rng, sides)
	}
	return faces
}

// Die throws one die of sides faces, one draw from rng.
func Die(rng *rand.Rand, sides int) Face { return Face(rng.IntN(sides) + 1) }
