package poker

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 20000

func users(n int) []game.UserID {
	return []game.UserID{"alice", "bob", "carol", "dave", "erin", "frank"}[:n]
}

// choose is a deterministic player that mixes folds, checks, calls, raises
// and the odd all-in, and folds to big bets more often than not.
func choose(v View, seat game.Seat) Action {
	n, _ := strconv.Atoi(strings.TrimPrefix(string(seat), seatPrefix))
	me := v.Seats[n-1]
	key := (v.HandNo*7 + n*3 + len(v.Board)) % 11
	canRaise := slices.ContainsFunc(v.Legal, raisesTo)
	smallRaise := key%4 == 1 && canRaise && v.MinRaiseTo <= 3*v.Blinds.Big
	text := string(MoveCall)
	switch {
	case v.ToCall*4 > me.Stack && key%3 != 0:
		text = string(MoveFold)
	case key == 2 && canRaise:
		text = string(MoveAllIn)
	case smallRaise && slices.Contains(v.Legal, MoveBet):
		text = fmt.Sprintf("bet %d", v.MinRaiseTo)
	case smallRaise:
		text = fmt.Sprintf("raise %d", v.MinRaiseTo)
	case !slices.Contains(v.Legal, MoveCall):
		text = string(MoveCheck)
	}
	return Action{Kind: ActionAct, Text: text}
}

// playToFinish plays choose for every seat with the rng each step gets in the
// server, and returns the steps and every state from the first to the final one.
func playToFinish(t *testing.T, players []game.UserID, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	st, _, err := Rules{}.New(Opts{}, players[0], players[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	var steps []gametest.Step[Action]
	states := []state{st.(state)}
	for seq := game.Seq(1); len(st.ToAct()) > 0; seq++ {
		if seq > maxGameSteps {
			t.Fatalf("%d players: no finish within %d steps", len(players), maxGameSteps)
		}
		seat := st.ToAct()[0]
		action := choose(st.View(seat), seat)
		if st, _, err = st.Apply(seat, action, game.StepRNG(seed, seq)); err != nil {
			t.Fatalf("seq %d %s %q: %v", seq, seat, action.Text, err)
		}
		steps = append(steps, gametest.Step[Action]{Seat: seat, Action: action})
		states = append(states, st.(state))
	}
	return steps, states
}

func holes(h hand) []cards.Card {
	var all []cards.Card
	for i, dealt := range h.dealtIn {
		if dealt {
			all = append(all, h.holes[i][:]...)
		}
	}
	return all
}

// showedDown is whether h ended with its contested hands revealed. It reads
// only who still contested the pot, not what the rules chose to show.
func showedDown(h hand) bool { return h.chips.contestants() > 1 }

// endedPublic is what the last hand legitimately made public: its board, and
// the contested hands when it reached a showdown.
func endedPublic(s state) []cards.Card {
	ended := s.ended.hand
	if ended.number == 0 {
		return nil
	}
	public := slices.Clone(ended.board[:ended.dealt])
	for i := range s.n {
		if showedDown(ended) && ended.chips.contesting[i] {
			public = append(public, ended.holes[i][:]...)
		}
	}
	return public
}

// undealt is the rest of the current hand's deck: no seat may see it yet.
func undealt(s state) []cards.Card {
	deck := cards.Shuffled(cards.French(), s.subseed, s.hand.number)
	seen := append(holes(s.hand), s.hand.board[:s.hand.dealt]...)
	return slices.DeleteFunc(deck, func(c cards.Card) bool { return slices.Contains(seen, c) })
}

func without(from, drop []cards.Card) []cards.Card {
	return slices.DeleteFunc(slices.Clone(from), func(c cards.Card) bool { return slices.Contains(drop, c) })
}

// ranUnplayed is whether h ended with no move in its last round: every seat
// was all-in from the blinds. Such a hand runs out inside the step that dealt
// it, so that step's events may also hold earlier hands this state no longer knows.
func ranUnplayed(h hand) bool { return !slices.Contains(h.chips.acted[:h.chips.n], true) }

// privateCards is what seat alone may see, judged on the state before a step
// as well as after it. Before it: the hand that just ended, whose cards stay
// private unless it showed down with seat contesting. After it: seat's
// current hole cards. And the undealt deck, card by card, is private to
// every seat, which bars it from every view. A card string can recur in
// another hand, so the cards a seat may legitimately see are left out; after
// a hand that ran unplayed, which cards those are is unknown, and nothing is
// checked until the next hand ends.
func privateCards(s state, seat game.Seat) []string {
	i, ok := s.seatIndex(seat)
	if !ok || (s.ended.hand.number != 0 && ranUnplayed(s.ended.hand)) {
		return nil
	}
	public := endedPublic(s)
	var private []cards.Card
	if s.hand.dealtIn[i] {
		private = append(private, without(s.hand.holes[i][:], public)...)
	}
	ended := s.ended.hand
	if revealed := showedDown(ended) && ended.chips.contesting[i]; ended.number != 0 && ended.dealtIn[i] && !revealed {
		visible := append(holes(s.hand), s.hand.board[:s.hand.dealt]...)
		private = append(private, without(ended.holes[i][:], visible)...)
	}
	private = append(private, without(undealt(s), public)...)
	out := make([]string, len(private))
	for k, c := range private {
		out[k] = c.String()
	}
	return out
}

// subseed is the seed every deck comes from.
func subseed(t *testing.T, players []game.UserID, seed game.Seed) string {
	t.Helper()
	st, _, err := Rules{}.New(Opts{}, players[0], players[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return strconv.FormatUint(st.(state).subseed, 10)
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := minPlayers; n <= maxPlayers; n++ {
		for _, seed := range []game.Seed{3, 8} {
			players := users(n)
			steps, states := playToFinish(t, players, seed)
			first, _, _ := Rules{}.New(Opts{}, players[0], players[1:], game.StepRNG(seed, 0))
			toAct := first.ToAct()[0]
			illegal := []gametest.Step[Action]{
				{Seat: seatName((first.(state).hand.toAct + 1) % n), Action: Action{Kind: ActionAct, Text: "call"}, WantErr: true},
				{Seat: toAct, Action: Action{Kind: ActionAct, Text: "check"}, WantErr: true},
			}
			result, _ := states[len(states)-1].Outcome()
			scripts = append(scripts, gametest.Script[Opts, Action]{
				Name:       fmt.Sprintf("%d players seed %d", n, seed),
				Challenger: players[0],
				Invitees:   players[1:],
				Seed:       seed,
				Steps:      append(illegal, steps...),
				WantResult: &game.Result{Kind: game.ResultDecisive, Winners: result.Winners, Reason: ReasonAllChips},
				PrivateOf:  gametest.Private(privateCards),
				Secrets:    []string{subseed(t, players, seed)},
			})
		}
	}
	gametest.Run(t, Rules{}, scripts)
}

func TestFinishedGamesScoreChipDeltasThatSumToZero(t *testing.T) {
	for n := minPlayers; n <= maxPlayers; n++ {
		_, states := playToFinish(t, users(n), 5)
		final := states[len(states)-1]
		var sum int64
		for _, score := range final.Scores() {
			sum += score
		}
		winner := final.result.Winners[0]
		if sum != 0 || final.Scores()[winner] != int64(n-1)*int64(startingStack) {
			t.Errorf("%d players: scores %v sum to %d", n, final.Scores(), sum)
		}
	}
}

// TestViewLegalityIsWhatApplyAccepts holds the view to Apply: the seat to act
// may make a move exactly when Legal lists it, with a bet or raise amount
// inside [MinRaiseTo, MaxRaiseTo], and every other seat is offered nothing.
func TestViewLegalityIsWhatApplyAccepts(t *testing.T) {
	for n := minPlayers; n <= maxPlayers; n++ {
		_, states := playToFinish(t, users(n), 3)
		for _, s := range states[:len(states)-1] {
			toAct := s.ToAct()[0]
			for i := range s.n {
				if v := s.View(seatName(i)); seatName(i) != toAct && (len(v.Legal) > 0 || v.ToCall != 0 || v.MaxRaiseTo != 0) {
					t.Fatalf("%d players hand %d: %s is not to act but is offered %v", n, s.hand.number, seatName(i), v.Legal)
				}
			}
			v := s.View(toAct)
			for _, text := range probeMoves(v) {
				m, _ := parseMove(text)
				_, _, err := s.Apply(toAct, Action{Kind: ActionAct, Text: text}, nil)
				if offered := offers(v, m); offered != (err == nil) {
					t.Fatalf("%d players hand %d %s %q: offered %t, Apply: %v; legal %v range %d..%d",
						n, s.hand.number, toAct, text, offered, err, v.Legal, v.MinRaiseTo, v.MaxRaiseTo)
				}
			}
		}
	}
}

// probeMoves are every move kind, with bet and raise amounts on and just past each end of the view's range.
func probeMoves(v View) []string {
	texts := []string{string(MoveFold), string(MoveCheck), string(MoveCall), string(MoveAllIn)}
	for _, kind := range []MoveKind{MoveBet, MoveRaise} {
		for _, amount := range []Chips{v.Blinds.Big, v.MinRaiseTo - 1, v.MinRaiseTo, v.MaxRaiseTo, v.MaxRaiseTo + 1} {
			if amount > 0 {
				texts = append(texts, fmt.Sprintf("%s %d", kind, amount))
			}
		}
	}
	return texts
}

// offers is whether v lets its seat play m; a call with nothing to call is the check it plays as.
func offers(v View, m move) bool {
	listed := slices.Contains(v.Legal, m.kind) || (m.kind == MoveCall && slices.Contains(v.Legal, MoveCheck))
	if raisesTo(m.kind) {
		return listed && v.MinRaiseTo <= m.amount && m.amount <= v.MaxRaiseTo
	}
	return listed
}
