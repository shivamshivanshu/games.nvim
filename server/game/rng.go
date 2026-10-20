package game

import "math/rand/v2"

func StepRNG(seed, seq uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seq))
}
