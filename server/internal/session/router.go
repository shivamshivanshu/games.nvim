package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

var errUnauthenticated = protocol.Error{Code: protocol.CodeUnauthenticated, Msg: "login required"}

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
		return protocol.BadRequestf("malformed body: %v", err)
	}
	return nil
}

func errorBody(err error) protocol.ErrorBody {
	if e, ok := errors.AsType[protocol.Error](err); ok {
		return protocol.ErrorBody{Code: e.Code, Message: e.Msg}
	}
	if e, ok := errors.AsType[game.BadRequestError](err); ok {
		return protocol.ErrorBody{Code: protocol.CodeBadRequest, Message: e.Msg}
	}
	if e, ok := errors.AsType[game.IllegalActionError](err); ok {
		return protocol.ErrorBody{Code: protocol.CodeIllegalAction, Message: e.Msg}
	}
	if errors.Is(err, game.ErrNotYourTurn) {
		return protocol.ErrorBody{Code: protocol.CodeNotYourTurn, Message: err.Error()}
	}
	slog.Error("session: request failed", "err", err)
	return protocol.ErrorBody{Code: protocol.CodeInternal, Message: internalErrorMessage}
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
