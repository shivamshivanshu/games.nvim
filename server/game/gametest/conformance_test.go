package gametest

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// failureRecorder is a testing.TB that records failures instead of failing the real test.
type failureRecorder struct {
	testing.TB
	failures []string
}

func (*failureRecorder) Helper() {}

func (r *failureRecorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *failureRecorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func recordFailures(check func(testing.TB)) []string {
	r := &failureRecorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		check(r)
	}()
	<-done
	return r.failures
}

func tableScript(players int) Script[tableOpts, tableAction] {
	invitees := make([]game.UserID, players-1)
	for i := range invitees {
		invitees[i] = game.UserID(fmt.Sprintf("u%d", i+2))
	}
	return Script[tableOpts, tableAction]{
		Name:       fmt.Sprintf("%d players", players),
		Challenger: "u1",
		Invitees:   invitees,
		Seed:       1,
		Steps: []Step[tableAction]{
			{Seat: "s1", Action: tableAction{Kind: tableSay}},
			{Seat: "s1", Action: tableAction{Kind: tableSay, Finish: true}},
		},
		PrivateOf: Private(privateHands),
		Secrets:   []string{tableSecret, escapedSecret},
	}
}

func TestRunAcceptsConformingGames(t *testing.T) {
	for _, tc := range []struct{ players, max int }{{1, 1}, {2, 2}, {3, 4}} {
		Run(t, tableRules{max: tc.max}, []Script[tableOpts, tableAction]{tableScript(tc.players)})
	}
}

func TestPrivateRejectsAnotherStateType(t *testing.T) {
	failures := recordFailures(func(tb testing.TB) {
		conform(tb, CounterRules{}, Script[CounterOpts, CounterAction]{Challenger: "u", PrivateOf: Private(privateHands)})
	})
	want := "PrivateOf reads gametest.tableState, but the game state is gametest.counterState"
	if !slices.Contains(failures, want) {
		t.Errorf("want failure %q, got %q", want, failures)
	}
}

func TestRunRejectsNonconformingGames(t *testing.T) {
	tests := []struct {
		name         string
		players, max int
		flaw         flaw
		want         string
	}{
		{"private hand in another seat's view", 2, 2, leakPrivateInView, `s2's view, private to s1 leaks "hand-s1-r0"`},
		{"private hand as a view map key", 2, 2, leakPrivateInViewKey, `s2's view, private to s1 leaks "hand-s1-r0"`},
		{"private hand in an event field", 2, 2, leakPrivateInEvent, `events leaks "hand-s1-r1"`},
		{"private hand as an event kind", 2, 2, leakPrivateInEventKind, `events leaks "hand-s1-r1"`},
		{"secret in a view before finish", 2, 2, leakSecretInView, `view leaks "answer-42"`},
		{"secret that JSON escapes, in a view", 2, 2, leakEscapedSecretInView, `view leaks "a<b&\"c"`},
		{"secret in an event before finish", 2, 2, leakSecretInEvent, `events leaks "answer-42"`},
		{"finishes with two seats left", 3, 4, finishOnFirstElimination, "play must continue while 2 or more remain"},
		{"solo game survives eliminating its only seat", 1, 1, ignoreElimination, "only seat must finish a solo game"},
		{"two-player game survives an elimination", 2, 2, ignoreElimination, "left one seat but did not finish"},
		{"eliminated seat stays to act", 3, 4, ignoreElimination, "eliminated s1 is still in ToAct"},
		{"eliminating rewrites a result reached by elimination", 2, 2, eliminateRewritesFinished, "after eliminating s1: Eliminate(s2) on a finished game must be a no-op"},
		{"eliminating rewrites a result reached by play", 2, 2, eliminateRewritesPlayedResult, "after the script: Eliminate(s1) on a finished game must be a no-op"},
		{"an imposed ending leaves the game playing", 2, 2, concludeKeepsPlaying, "fresh from New: Conclude"},
		{"an event kind its Info does not list", 2, 2, uncataloguedEvent, `observation 1: event 1 of kind "shrugged" is not in Info.Events`},
		{"a clock without a time control", 2, 2, clockWithoutTimeControl, `Info: game table: the "clock" capability`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := tableRules{max: tt.max, flaw: tt.flaw}
			failures := recordFailures(func(tb testing.TB) { conform(tb, rules, tableScript(tt.players)) })
			if !slices.ContainsFunc(failures, func(f string) bool { return strings.Contains(f, tt.want) }) {
				t.Errorf("want a failure containing %q, got %q", tt.want, failures)
			}
		})
	}
}

func TestRunRejectsAnActionKindItsInfoDoesNotList(t *testing.T) {
	script := tableScript(2)
	script.Steps[0].Action.Kind = "wave"
	failures := recordFailures(func(tb testing.TB) { conform(tb, tableRules{max: 2}, script) })
	if want := `step 0: action kind "wave" is not in Info.Actions`; !slices.Contains(failures, want) {
		t.Errorf("want failure %q, got %q", want, failures)
	}
}
