package game

// Ending is embedded by a state to record how its game ended, giving it
// Outcome; the first result concluded is the one kept.
type Ending struct {
	result Result
	over   bool
}

func (e Ending) Outcome() (Result, bool) { return e.result, e.over }

func (e Ending) Over() bool { return e.over }

// Concluded is e ended on r, or e itself if it had already ended.
func (e Ending) Concluded(r Result) Ending {
	if e.over {
		return e
	}
	return Ending{result: r, over: true}
}

// ToActOf is seats while the game is on, none once it ended.
func (e Ending) ToActOf(seats ...Seat) []Seat {
	if e.over {
		return nil
	}
	return seats
}
