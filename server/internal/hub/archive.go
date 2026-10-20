package hub

import (
	"context"
	"errors"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/archive"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/room"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

var errSummaryMismatch = protocol.Error{Code: protocol.CodeSummaryMismatch, Msg: "summary does not match the game's stored digest"}

func finishedUpdate(rec store.GameRecord, user game.UserID) (protocol.GameUpdate, error) {
	if !slices.ContainsFunc(rec.Players, func(p game.Player) bool { return p.User == user }) {
		return protocol.GameUpdate{}, room.ErrNotAPlayer
	}
	finished, err := archive.FinishedOf(rec)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	return finished.Update()
}

// verify checks a client's archived summary against the digest stored at
// finish, so a client cannot alter a past game's outcome. Every failure is
// the same mismatch, so verify does not reveal which game ids exist.
func (h *Hub) verify(ctx context.Context, _ *session.Session, req protocol.GameVerifyRequest) (protocol.Empty, error) {
	raw := []byte(req.Summary)
	id, err := archive.GameIDOf(raw)
	if err != nil {
		return protocol.Empty{}, errSummaryMismatch
	}
	stored, err := h.deps.Store.Digest(ctx, id)
	switch {
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrNotFinished):
		return protocol.Empty{}, errSummaryMismatch
	case err != nil:
		return protocol.Empty{}, err
	case archive.Sum(raw) != stored:
		return protocol.Empty{}, errSummaryMismatch
	}
	return protocol.Empty{}, nil
}
