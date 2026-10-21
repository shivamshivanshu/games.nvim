package yahtzee

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
)

// TypeYahtzee is the game type on the wire.
const TypeYahtzee game.GameType = "yahtzee"

// The dice and turns of a game.
const (
	DiceCount    = 5
	RollsPerTurn = 3
	Rounds       = boxCount
)

// Opts is empty: Yahtzee has no options.
type Opts struct{}

// Box is a scorecard box.
type Box string

// The boxes, upper section then lower, in scorecard order.
const (
	BoxOnes          Box = "ones"
	BoxTwos          Box = "twos"
	BoxThrees        Box = "threes"
	BoxFours         Box = "fours"
	BoxFives         Box = "fives"
	BoxSixes         Box = "sixes"
	BoxThreeOfAKind  Box = "three_of_a_kind"
	BoxFourOfAKind   Box = "four_of_a_kind"
	BoxFullHouse     Box = "full_house"
	BoxSmallStraight Box = "small_straight"
	BoxLargeStraight Box = "large_straight"
	BoxYahtzee       Box = "yahtzee"
	BoxChance        Box = "chance"
)

// The action kinds.
const (
	ActionRoll  game.ActionKind = "roll"
	ActionHold  game.ActionKind = "hold"
	ActionScore game.ActionKind = "score"
)

// Action is a roll of the dice not held, a hold toggled on die Die (1 to
// DiceCount), or the dice scored in Box.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Die  string          `json:"die,omitempty"`
	Box  string          `json:"box,omitempty"`
}

// View is the table, the same for every seat but for what the seat to act may do.
//
// Round runs from 1 to Rounds. Dice is empty before the turn's first roll.
// CanRoll, CanHold and Options are the viewer's own: false and empty unless
// it is to act. Options lists the boxes it may score now, with what each
// would get; under the joker rule that is only the boxes the rule allows.
// BonusAt is the upper total that earns the upper bonus.
type View struct {
	Round     int         `json:"round"`
	Rounds    int         `json:"rounds"`
	Dice      []dice.Face `json:"dice"`
	Held      []bool      `json:"held"`
	RollsLeft int         `json:"rolls_left"`
	Cards     []Card      `json:"cards"`
	CanRoll   bool        `json:"can_roll"`
	CanHold   bool        `json:"can_hold"`
	Options   []Option    `json:"options"`
	BonusAt   int         `json:"bonus_at"`
}

// Card is one seat's scorecard, in seat order. Scores holds the filled boxes
// alone. Out is a seat that left: its card stays, and it takes no more turns.
type Card struct {
	Seat         game.Seat   `json:"seat"`
	Scores       map[Box]int `json:"scores"`
	UpperTotal   int         `json:"upper_total"`
	UpperBonus   int         `json:"upper_bonus"`
	YahtzeeBonus int         `json:"yahtzee_bonus"`
	Total        int         `json:"total"`
	Out          bool        `json:"out"`
}

// Option is a box the seat to act may score, and what it would score there now.
type Option struct {
	Box   Box `json:"box"`
	Score int `json:"score"`
}

// ReasonScorecardsFull ends the game when every seat still playing has filled its card.
const ReasonScorecardsFull game.ResultReason = "scorecards_full"

// The event kinds.
const (
	EventRolled game.EventKind = "rolled"
	EventScored game.EventKind = "scored"
)

// RolledEvent reports a roll: Dice are all five after it, held ones included.
type RolledEvent struct {
	By   game.Seat   `json:"by"`
	Dice []dice.Face `json:"dice"`
}

// ScoredEvent reports a box filled, and the Yahtzee bonus the roll earned on top, 0 or 100.
type ScoredEvent struct {
	By           game.Seat `json:"by"`
	Box          Box       `json:"box"`
	Score        int       `json:"score"`
	YahtzeeBonus int       `json:"yahtzee_bonus"`
}

// EventKind is EventRolled.
func (RolledEvent) EventKind() game.EventKind { return EventRolled }

// EventKind is EventScored.
func (ScoredEvent) EventKind() game.EventKind { return EventScored }
