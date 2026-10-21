package poker

import (
	"fmt"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// betting is the chips of one hand: what each seat has behind and in front,
// who still contests the pot, and the open betting round.
type betting struct {
	n          int
	stack      [maxPlayers]Chips
	bet        [maxPlayers]Chips
	committed  [maxPlayers]Chips
	contesting [maxPlayers]bool
	acted      [maxPlayers]bool
	// actedAt is the street's bet level when the seat last acted; it decides
	// whether later raises, together, reopen the betting to that seat.
	actedAt    [maxPlayers]Chips
	currentBet Chips
	increment  Chips
	bigBlind   Chips
}

// newStreet opens a betting round: no bet, and a raise must add at least the big blind.
func (b betting) newStreet() betting {
	b.bet, b.acted, b.actedAt = [maxPlayers]Chips{}, [maxPlayers]bool{}, [maxPlayers]Chips{}
	b.currentBet, b.increment = 0, b.bigBlind
	return b
}

func (b betting) commit(i int, amount Chips) betting {
	b.stack[i] -= amount
	b.bet[i] += amount
	b.committed[i] += amount
	return b
}

// post puts in a blind, or the whole stack when it is short; it is not an action.
func (b betting) post(i int, blind Chips) (betting, Chips) {
	amount := min(blind, b.stack[i])
	return b.commit(i, amount), amount
}

func (b betting) contestants() int {
	n := 0
	for i := range b.n {
		if b.contesting[i] {
			n++
		}
	}
	return n
}

func (b betting) canBet(i int) bool { return b.contesting[i] && b.stack[i] > 0 }

func (b betting) allIn(i int) bool { return b.contesting[i] && b.stack[i] == 0 }

func (b betting) bettors() int {
	n := 0
	for i := range b.n {
		if b.canBet(i) {
			n++
		}
	}
	return n
}

func (b betting) othersCanBet(i int) bool {
	for j := range b.n {
		if j != i && b.canBet(j) {
			return true
		}
	}
	return false
}

// matchable is the most seat i may have in this street: its whole stack,
// capped by the largest total an opponent still contesting has or can reach.
// Chips above it stay behind, so no seat commits chips that only it, or a
// seat that has folded or left, could contest.
func (b betting) matchable(i int) Chips {
	var reach Chips
	for j := range b.n {
		if j != i && b.contesting[j] {
			reach = max(reach, b.allInTotal(j))
		}
	}
	return min(b.allInTotal(i), reach)
}

// toCall is what seat i must add to stay in. It never exceeds what an
// opponent can put in this street, so a short all-in big blind is not
// matched beyond its size by the last seat with chips.
func (b betting) toCall(i int) Chips {
	return max(0, min(b.currentBet, b.matchable(i))-b.bet[i])
}

func (b betting) needsAction(i int) bool {
	return b.canBet(i) && (b.toCall(i) > 0 || (!b.acted[i] && b.othersCanBet(i)))
}

// nextToAct is the first seat from `from`, clockwise and inclusive, that must act.
func (b betting) nextToAct(from int) (int, bool) {
	for k := range b.n {
		if i := (from + k) % b.n; b.needsAction(i) {
			return i, true
		}
	}
	return 0, false
}

func (b betting) minRaiseTo() Chips { return b.currentBet + b.increment }

// raiseBlocked says why seat i may not raise. A seat that has acted may raise
// again only once the raises since then add up to a full raise.
func (b betting) raiseBlocked(i int) error {
	if !b.othersCanBet(i) {
		return game.IllegalActionError{Msg: "everyone else is all-in: call or fold"}
	}
	if b.acted[i] && b.currentBet-b.actedAt[i] < b.increment {
		return game.IllegalActionError{Msg: "betting was not reopened by a full raise: call or fold"}
	}
	return nil
}

func (b betting) allInTotal(i int) Chips { return b.bet[i] + b.stack[i] }

// legalMoves are the moves apply accepts from seat i as named: call is left
// out when it would be a check, and bet or raise is in when one to
// minRaiseTo is legal.
func (b betting) legalMoves(i int) []MoveKind {
	probes := []move{
		{kind: MoveFold}, {kind: MoveCheck}, {kind: MoveCall},
		{kind: MoveBet, amount: b.minRaiseTo()}, {kind: MoveRaise, amount: b.minRaiseTo()},
		{kind: MoveAllIn},
	}
	legal := []MoveKind{}
	for _, m := range probes {
		if _, played, err := b.apply(i, m); err == nil && played == m.kind {
			legal = append(legal, m.kind)
		}
	}
	return legal
}

func raisesTo(kind MoveKind) bool { return kind == MoveBet || kind == MoveRaise }

// apply plays seat i's move and returns the move as played: a call with
// nothing to call is a check.
func (b betting) apply(i int, m move) (betting, MoveKind, error) {
	played := m.kind
	var err error
	switch m.kind {
	case MoveFold:
		b.contesting[i] = false
	case MoveCheck:
		if owed := b.toCall(i); owed > 0 {
			return b, "", game.IllegalActionError{Msg: fmt.Sprintf("cannot check: %d to call", owed)}
		}
	case MoveCall:
		if b.toCall(i) == 0 {
			played = MoveCheck
		}
		b = b.commit(i, b.toCall(i))
	case MoveBet:
		if b.currentBet > 0 {
			return b, "", game.IllegalActionError{Msg: "a bet is already open: call, raise or fold"}
		}
		b, err = b.fullRaise(i, m)
	case MoveRaise:
		if b.currentBet == 0 {
			return b, "", game.IllegalActionError{Msg: "there is no bet to raise: bet instead"}
		}
		b, err = b.fullRaise(i, m)
	case MoveAllIn:
		b, err = b.allInMove(i)
	}
	if err != nil {
		return b, "", err
	}
	b.acted[i], b.actedAt[i] = true, b.currentBet
	return b, played, nil
}

func (b betting) fullRaise(i int, m move) (betting, error) {
	if err := b.raiseBlocked(i); err != nil {
		return b, err
	}
	if most := b.allInTotal(i); m.amount > most {
		return b, game.IllegalActionError{Msg: fmt.Sprintf("%s %d is more than your %d", m.kind, m.amount, most)}
	}
	if most := b.matchable(i); m.amount > most {
		return b, game.IllegalActionError{Msg: fmt.Sprintf("%s %d is more than any opponent can match: at most %d", m.kind, m.amount, most)}
	}
	if least := b.minRaiseTo(); m.amount < least {
		return b, game.IllegalActionError{Msg: fmt.Sprintf("the minimum %s is %d; allin for less", m.kind, least)}
	}
	return b.raiseTo(i, m.amount), nil
}

// allInMove commits the whole stack, or as much of it as an opponent can
// match. Below the bet it is a call for less; above it, a raise that changes
// the increment and reopens betting only when it is a full raise.
func (b betting) allInMove(i int) (betting, error) {
	total := b.matchable(i)
	if total <= b.currentBet {
		return b.commit(i, max(0, total-b.bet[i])), nil
	}
	if err := b.raiseBlocked(i); err != nil {
		return b, err
	}
	return b.raiseTo(i, total), nil
}

func (b betting) raiseTo(i int, total Chips) betting {
	if by := total - b.currentBet; by >= b.increment {
		b.increment = by
	}
	b = b.commit(i, total-b.bet[i])
	b.currentBet = total
	return b
}
