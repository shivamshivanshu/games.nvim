// Package archive turns a finished game into what the server keeps of it: its
// metadata, final views and exports, and a canonical summary whose digest
// makes the outcome tamper-evident. It knows no game.
package archive

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// Summary's field order is its byte layout, and Outcomes are sorted by user,
// so equal games always encode to equal bytes. Strings are not HTML-escaped,
// so the bytes can be rebuilt by any plain JSON encoder.
type Summary struct {
	GameID     game.GameID       `json:"game_id"`
	Game       game.GameType     `json:"game"`
	StartedAt  protocol.UnixMS   `json:"started_at"`
	FinishedAt protocol.UnixMS   `json:"finished_at"`
	Reason     game.ResultReason `json:"reason"`
	Outcomes   []Outcome         `json:"outcomes"`
}

type Outcome struct {
	User  game.UserID `json:"user"`
	Score int64       `json:"score"`
}

type Finished struct {
	ID        game.GameID
	Type      game.GameType
	Options   game.Options
	Players   []game.Player
	StartedAt time.Time
	Finish    store.Finish
}

func FinishedOf(rec store.GameRecord) (Finished, error) {
	if rec.Finish == nil {
		return Finished{}, fmt.Errorf("archive: game %s is not finished", rec.ID)
	}
	var opts game.Options
	if err := json.Unmarshal(rec.Options, &opts); err != nil {
		return Finished{}, fmt.Errorf("archive: game %s options: %w", rec.ID, err)
	}
	return Finished{
		ID: rec.ID, Type: rec.Type, Options: opts, Players: rec.Players,
		StartedAt: rec.CreatedAt, Finish: *rec.Finish,
	}, nil
}

func (f Finished) Summary() Summary {
	outcomes := xslices.Map(f.Players, func(p game.Player) Outcome {
		return Outcome{User: p.User, Score: f.Finish.Standings[p.User].Score}
	})
	slices.SortFunc(outcomes, func(a, b Outcome) int { return cmp.Compare(a.User, b.User) })
	return Summary{
		GameID:     f.ID,
		Game:       f.Type,
		StartedAt:  protocol.UnixMSOf(f.StartedAt),
		FinishedAt: protocol.UnixMSOf(f.Finish.At),
		Reason:     f.Finish.Reason,
		Outcomes:   outcomes,
	}
}

// Sealed returns f with its summary bytes and their digest filled in.
func (f Finished) Sealed() (Finished, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f.Summary()); err != nil {
		return Finished{}, fmt.Errorf("archive: encode summary of %s: %w", f.ID, err)
	}
	raw := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	f.Finish.Summary, f.Finish.Digest = raw, Sum(raw)
	return f, nil
}

// ViewedBy is the finished game as user sees it: Update with user's final view.
func (f Finished) ViewedBy(user game.UserID) protocol.GameUpdate {
	u := f.Update()
	u.View = f.Finish.Views[user]
	return u
}

// Update is the finished game as every player sees it, without a view.
func (f Finished) Update() protocol.GameUpdate {
	return protocol.GameUpdate{
		GameID:     f.ID,
		Game:       f.Type,
		Options:    f.Options,
		Seq:        f.Finish.Seq,
		Players:    append([]game.Player{}, f.Players...),
		ToAct:      []game.UserID{},
		Result:     &protocol.WireResult{Kind: f.Finish.Kind, Winners: f.winners(), Reason: f.Finish.Reason},
		Controls:   []protocol.Control{},
		Events:     []json.RawMessage{},
		StartedAt:  protocol.UnixMSOf(f.StartedAt),
		FinishedAt: new(protocol.UnixMSOf(f.Finish.At)),
		Summary:    new(string(f.Finish.Summary)),
		Digest:     new(f.Finish.Digest),
		Exports:    maps.Clone(f.Finish.Exports),
	}
}

func (f Finished) winners() []game.UserID {
	winners := []game.UserID{}
	for _, p := range f.Players {
		if f.Finish.Standings[p.User].Outcome == store.OutcomeWin {
			winners = append(winners, p.User)
		}
	}
	return winners
}

func Sum(raw []byte) game.Digest {
	sum := sha256.Sum256(raw)
	return game.Digest(hex.EncodeToString(sum[:]))
}

func GameIDOf(summary []byte) (game.GameID, error) {
	var s Summary
	if err := json.Unmarshal(summary, &s); err != nil {
		return "", fmt.Errorf("archive: malformed summary: %w", err)
	}
	return s.GameID, nil
}
