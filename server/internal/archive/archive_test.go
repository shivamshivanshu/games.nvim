package archive_test

import (
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/archive"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

func TestSealedSummaryIsCanonical(t *testing.T) {
	sealed, err := archive.Finished{
		ID: "g1", Type: "counter",
		Players:   []game.Player{{User: "zed", Seat: "a"}, {User: "a<&>b", Seat: "b"}},
		StartedAt: clocktest.Epoch,
		Finish: store.Finish{At: clocktest.Epoch.Add(2 * time.Second), Ending: store.Ending{
			Kind: game.ResultDecisive, Reason: game.ReasonResign,
			Standings: map[game.UserID]store.Standing{"zed": {Outcome: store.OutcomeWin, Score: 1}, "a<&>b": {Outcome: store.OutcomeLoss, Score: -1}},
		}},
	}.Sealed()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"game_id":"g1","game":"counter","started_at":1700000000000,"finished_at":1700000002000,"reason":"resign",` +
		`"outcomes":[{"user":"a<&>b","score":-1},{"user":"zed","score":1}]}`
	if got := string(sealed.Finish.Summary); got != want {
		t.Fatalf("summary\n got %s\nwant %s", got, want)
	}
	if sealed.Finish.Digest != archive.Sum([]byte(want)) {
		t.Fatalf("digest %s does not cover the summary bytes", sealed.Finish.Digest)
	}
}
