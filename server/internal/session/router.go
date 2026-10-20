package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/room"
	"games.nvim/server/internal/store"
)

var (
	ErrAuthFailed      = errors.New("authentication failed")
	errUnauthenticated = errors.New("login required")
	errVersionMismatch = errors.New("protocol version mismatch")
)

const internalErrorMessage = "internal error"

type Handler func(ctx context.Context, s *Session, body json.RawMessage) (any, error)

type Router map[protocol.MessageType]Handler

func Handle[Req, Resp any](fn func(ctx context.Context, s *Session, req Req) (Resp, error)) Handler {
	return func(ctx context.Context, s *Session, body json.RawMessage) (any, error) {
		var req Req
		if err := decodeBody(body, &req); err != nil {
			return nil, err
		}
		return fn(ctx, s, req)
	}
}

func decodeBody(body json.RawMessage, into any) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, into); err != nil {
		return game.BadRequestError{Msg: fmt.Sprintf("malformed body: %v", err)}
	}
	return nil
}

func errorBody(err error) protocol.ErrorBody {
	var badRequest game.BadRequestError
	var illegal game.IllegalActionError
	switch {
	case errors.As(err, &badRequest):
		return protocol.ErrorBody{Code: protocol.CodeBadRequest, Message: badRequest.Msg}
	case errors.As(err, &illegal):
		return protocol.ErrorBody{Code: protocol.CodeIllegalAction, Message: illegal.Msg}
	case errors.Is(err, game.ErrNotYourTurn):
		return protocol.ErrorBody{Code: protocol.CodeNotYourTurn, Message: err.Error()}
	case errors.Is(err, room.ErrStale):
		return protocol.ErrorBody{Code: protocol.CodeStale, Message: err.Error()}
	case errors.Is(err, store.ErrNotFound), errors.Is(err, room.ErrNotAPlayer):
		return protocol.ErrorBody{Code: protocol.CodeNotFound, Message: err.Error()}
	case errors.Is(err, ErrAuthFailed):
		return protocol.ErrorBody{Code: protocol.CodeAuthFailed, Message: err.Error()}
	case errors.Is(err, errVersionMismatch):
		return protocol.ErrorBody{Code: protocol.CodeVersionMismatch, Message: err.Error()}
	case errors.Is(err, errUnauthenticated):
		return protocol.ErrorBody{Code: protocol.CodeUnauthenticated, Message: err.Error()}
	default:
		slog.Error("session: request failed", "err", err)
		return protocol.ErrorBody{Code: protocol.CodeInternal, Message: internalErrorMessage}
	}
}

func errorEnvelope(id *protocol.RequestID, body protocol.ErrorBody) protocol.Envelope {
	raw, _ := json.Marshal(body)
	return protocol.Envelope{ID: id, Type: protocol.TypeError, Body: raw}
}

func replyEnvelope(id *protocol.RequestID, result any, err error) protocol.Envelope {
	if err != nil {
		return errorEnvelope(id, errorBody(err))
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return errorEnvelope(id, errorBody(fmt.Errorf("session: encode result: %w", err)))
	}
	return protocol.Envelope{ID: id, Type: protocol.TypeOK, Body: raw}
}
