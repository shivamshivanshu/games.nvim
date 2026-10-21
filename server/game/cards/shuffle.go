// Package cards is what card games share: a deck shuffled from a game's
// sub-seed, and the 52-card French deck with its two-letter wire text.
package cards

import "github.com/shivamshivanshu/games.nvim/server/game"

// Shuffled is a shuffled copy of ordered, the n-th deck a game with this
// sub-seed shuffles. It depends on nothing else, so a state can shuffle where
// it gets no rng, such as in Eliminate, exactly as a replay will.
func Shuffled[C any](ordered []C, subseed uint64, n int) []C {
	deck := make([]C, len(ordered))
	copy(deck, ordered)
	rng := game.StepRNG(game.Seed(subseed), game.Seq(n))
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	return deck
}
