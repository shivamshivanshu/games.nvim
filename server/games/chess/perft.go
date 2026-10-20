package chess

func Perft(p Position, depth int) uint64 {
	if depth == 0 {
		return 1
	}
	moves := p.LegalMoves()
	if depth == 1 {
		return uint64(len(moves))
	}
	var nodes uint64
	for _, m := range moves {
		nodes += Perft(p.Play(m), depth-1)
	}
	return nodes
}
