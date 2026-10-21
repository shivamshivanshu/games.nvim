package poker

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

// TypePoker is the game type on the wire.
const TypePoker game.GameType = "poker"

// Chips is an amount of chips.
type Chips int64

const (
	minPlayers    = 2
	maxPlayers    = 6
	handsPerLevel = 10

	startingStack    Chips = 1000
	firstSmallBlind  Chips = 10
	firstBigBlind    Chips = 20
	holeCardsPerSeat       = 2
	boardSize              = 5
)

// ReasonAllChips ends a game in which one seat has won every chip in play.
const ReasonAllChips game.ResultReason = "all_chips"

// Opts is empty: the stack, blinds and schedule are fixed (D13).
type Opts struct{}

// ActionAct carries a betting move typed by the player; it is poker's only action.
const ActionAct game.ActionKind = "act"

const actionTextField = "text"

// movePattern is what parseMove accepts, for clients to check input before sending it.
const movePattern = `(?i)^\s*(fold|check|call|allin|(bet|raise)\s+[0-9]+)\s*$`

// Action is a betting move as typed: fold, check, call, bet N, raise N or allin.
// Amounts are the seat's total for the street ("raise to").
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Text string          `json:"text"`
}

// MoveKind is a parsed betting move.
type MoveKind string

// The betting moves.
const (
	MoveFold  MoveKind = "fold"
	MoveCheck MoveKind = "check"
	MoveCall  MoveKind = "call"
	MoveBet   MoveKind = "bet"
	MoveRaise MoveKind = "raise"
	MoveAllIn MoveKind = "allin"
)

// Street is a betting round, named by the board cards dealt before it.
type Street string

// The streets after the preflop round, each named by the cards it deals.
const (
	StreetFlop  Street = "flop"
	StreetTurn  Street = "turn"
	StreetRiver Street = "river"
)

// BlindKind tells the small blind from the big blind.
type BlindKind string

// The two blinds.
const (
	BlindSmall BlindKind = "small"
	BlindBig   BlindKind = "big"
)

// Blinds are the forced bets of a hand.
type Blinds struct {
	Small Chips `json:"small"`
	Big   Chips `json:"big"`
}

// View is the table as one seat sees it: only its own hole cards, until a showdown reveals others in LastHand.
//
// Legal, ToCall and the raise range are given to the seat to act alone; every
// other viewer gets none and zeros. MinRaiseTo and MaxRaiseTo bound N in
// "bet N" or "raise N", a street total, when Legal holds bet or raise, and are
// 0 otherwise. MaxRaiseTo is what allin commits: the seat's whole stack, or
// less when no opponent still in the hand can match more.
type View struct {
	HandNo     int          `json:"hand_no"`
	Button     game.Seat    `json:"button"`
	Blinds     Blinds       `json:"blinds"`
	Board      []cards.Card `json:"board"`
	Pots       []PotView    `json:"pots"`
	Seats      []SeatView   `json:"seats"`
	Legal      []MoveKind   `json:"legal"`
	ToCall     Chips        `json:"to_call"`
	MinRaiseTo Chips        `json:"min_raise_to"`
	MaxRaiseTo Chips        `json:"max_raise_to"`
	MyCards    []cards.Card `json:"my_cards"`
	LastHand   *HandSummary `json:"last_hand" jsonschema:"nullable"`
}

// SeatView is one seat at the table, in table order.
type SeatView struct {
	Seat   game.Seat   `json:"seat"`
	User   game.UserID `json:"user"`
	Stack  Chips       `json:"stack"`
	Bet    Chips       `json:"bet"`
	Folded bool        `json:"folded"`
	AllIn  bool        `json:"allin"`
	Out    bool        `json:"out"`
}

// PotView is a settled pot from earlier streets; current bets are in SeatView.Bet.
type PotView struct {
	Amount   Chips       `json:"amount"`
	Eligible []game.Seat `json:"eligible"`
}

// HandSummary is how the previous hand ended: its board, the hands shown at
// showdown (none when everyone else folded) and the pots won.
type HandSummary struct {
	HandNo int          `json:"hand_no"`
	Board  []cards.Card `json:"board"`
	Shown  []ShownHand  `json:"shown"`
	Awards []Award      `json:"awards"`
}

// ShownHand is a hand revealed at showdown.
type ShownHand struct {
	Seat     game.Seat    `json:"seat"`
	Cards    []cards.Card `json:"cards"`
	Category Category     `json:"category"`
}

func cloneShown(hands []ShownHand) []ShownHand {
	out := make([]ShownHand, len(hands))
	for i, h := range hands {
		h.Cards = slices.Clone(h.Cards)
		out[i] = h
	}
	return out
}

// Award is a pot, or a seat's share of a split pot. Pot 0 is the main pot.
type Award struct {
	Seat   game.Seat `json:"seat"`
	Amount Chips     `json:"amount"`
	Pot    int       `json:"pot"`
}
