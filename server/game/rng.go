package game

import "math/rand/v2"

func StepRNG(seed Seed, seq Seq) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), uint64(seq)))
}
