package liarsdice

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
)

// TypeLiarsDice is the game type on the wire.
const TypeLiarsDice game.GameType = "liars_dice"

// StartingDice is the dice each seat starts with.
const StartingDice = 5

// Wild is the face that counts toward every other face's bid.
const Wild dice.Face = 1

// Opts is empty: Liar's Dice has no options.
type Opts struct{}

// The action kinds.
const (
	ActionBid  game.ActionKind = "bid"
	ActionDudo game.ActionKind = "dudo"
)

// Action is a bid typed as quantity x face, e.g. "3x5", or dudo, the
// challenge of the standing bid.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Bid  string          `json:"bid,omitempty"`
}

// Bid is By's claim that at least Quantity dice on the table show Face, ones
// counting as any face but when the bid is on ones.
type Bid struct {
	By       game.Seat `json:"by"`
	Quantity int       `json:"quantity"`
	Face     dice.Face `json:"face"`
}

// View is the table as one seat sees it: its own dice, and every other cup
// only as a count until a challenge reveals them.
//
// Bid is the standing bid, null when the round's bidding has not opened.
// MinBids and CanDudo are the viewer's own, empty and false unless it is to
// act: for each face it may bid now, the smallest quantity it may bid; any
// quantity from that up to TotalDice is legal. LastReveal is the previous
// round's challenge, with every cup it showed.
type View struct {
	Round      int          `json:"round"`
	Seats      []SeatView   `json:"seats"`
	Bid        *Bid         `json:"bid" jsonschema:"nullable"`
	TotalDice  int          `json:"total_dice"`
	MyDice     []dice.Face  `json:"my_dice"`
	MinBids    []BidMinimum `json:"min_bids"`
	CanDudo    bool         `json:"can_dudo"`
	LastReveal *Reveal      `json:"last_reveal" jsonschema:"nullable"`
}

// SeatView is one seat at the table, in seat order. Out is a seat with no
// dice left, whether lost or left the game.
type SeatView struct {
	Seat game.Seat `json:"seat"`
	Dice int       `json:"dice"`
	Out  bool      `json:"out"`
}

// BidMinimum is the smallest quantity the seat to act may bid on Face.
type BidMinimum struct {
	Face     dice.Face `json:"face"`
	Quantity int       `json:"quantity"`
}

// Reveal is how a challenge went: the bid, the challenger, how many dice
// counted toward it, who lost a die for it, and every cup in play.
type Reveal struct {
	Round      int        `json:"round"`
	Bid        Bid        `json:"bid"`
	Challenger game.Seat  `json:"challenger"`
	Counted    int        `json:"counted"`
	Loser      game.Seat  `json:"loser"`
	Cups       []SeatDice `json:"cups"`
}

// SeatDice is one seat's dice.
type SeatDice struct {
	Seat game.Seat   `json:"seat"`
	Dice []dice.Face `json:"dice"`
}

// ReasonLastWithDice ends the game when one seat alone has dice left.
const ReasonLastWithDice game.ResultReason = "last_with_dice"

// The event kinds.
const (
	EventBid        game.EventKind = "bid"
	EventChallenged game.EventKind = "challenged"
	EventOut        game.EventKind = "out"
)

// BidEvent reports a bid.
type BidEvent struct {
	By       game.Seat `json:"by"`
	Quantity int       `json:"quantity"`
	Face     dice.Face `json:"face"`
}

// ChallengedEvent reports By's dudo and the reveal it made.
type ChallengedEvent struct {
	By      game.Seat  `json:"by"`
	Bid     Bid        `json:"bid"`
	Counted int        `json:"counted"`
	Loser   game.Seat  `json:"loser"`
	Cups    []SeatDice `json:"cups"`
}

// OutEvent reports a seat that lost its last die.
type OutEvent struct {
	Seat game.Seat `json:"seat"`
}

// EventKind is EventBid.
func (BidEvent) EventKind() game.EventKind { return EventBid }

// EventKind is EventChallenged.
func (ChallengedEvent) EventKind() game.EventKind { return EventChallenged }

// EventKind is EventOut.
func (OutEvent) EventKind() game.EventKind { return EventOut }
