package room

import (
	"encoding/json"
	"fmt"
	"slices"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/store"
)

func (g setup) can(c game.Capability) bool { return slices.Contains(g.caps, c) }

func (s roomState) controls(g setup, user game.UserID) []protocol.Control {
	controls := []protocol.Control{}
	if s.finished() {
		return controls
	}
	if g.can(game.CapResign) {
		controls = append(controls, protocol.ControlResign)
	}
	if g.can(game.CapDraw) && !slices.Contains(s.drawAgreed, user) {
		controls = append(controls, protocol.ControlOfferDraw)
	}
	if len(s.drawAgreed) > 0 {
		controls = append(controls, protocol.ControlDeclineDraw)
	}
	if g.can(game.CapAbort) && len(s.acted) < len(g.Players) {
		controls = append(controls, protocol.ControlAbort)
	}
	return controls
}

func (s roomState) applyControl(g setup, rec store.StepRecord) (roomState, []game.Event, error) {
	var c protocol.Control
	if err := json.Unmarshal(rec.Payload, &c); err != nil {
		return s, nil, game.BadRequestError{Msg: fmt.Sprintf("malformed control: %v", err)}
	}
	if !slices.Contains(s.controls(g, rec.User), c) {
		return s, nil, game.IllegalActionError{Msg: "control not available: " + string(c)}
	}
	switch c {
	case protocol.ControlResign:
		state, events := s.game.Eliminate(g.seats[rec.User], game.ReasonResign)
		s.game = state
		return s, events, nil
	case protocol.ControlOfferDraw:
		s.drawAgreed = append(slices.Clip(s.drawAgreed), rec.User)
		if len(s.drawAgreed) == len(g.Players) {
			s.imposed = &game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
		}
	case protocol.ControlDeclineDraw:
		s.drawAgreed = nil
	case protocol.ControlAbort:
		s.imposed = &game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	}
	return s, nil, nil
}
