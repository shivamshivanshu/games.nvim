package game

// NextSeat is the first index after from, round a table of n seats, at which
// in holds, from itself coming last; it is from when in holds nowhere.
func NextSeat(n, from int, in func(i int) bool) int {
	for k := 1; k <= n; k++ {
		if i := (from + k) % n; in(i) {
			return i
		}
	}
	return from
}
