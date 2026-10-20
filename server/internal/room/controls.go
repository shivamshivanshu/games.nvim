package room

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

func (g Setup) can(c game.Capability) bool { return slices.Contains(g.info.Capabilities, c) }

func (s roomState) controls(g Setup, user game.UserID) []protocol.Control {
	controls := []protocol.Control{}
	if s.finished() || s.isEliminated(user) {
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
	if g.can(game.CapAbort) && !s.allActiveActed(g) {
		controls = append(controls, protocol.ControlAbort)
	}
	return controls
}

func (s roomState) applyControl(g Setup, st step) (roomState, error) {
	c := st.control
	if !slices.Contains(s.controls(g, st.user), c) {
		return s, protocol.Error{Code: protocol.CodeIllegalAction, Msg: "control not available: " + string(c)}
	}
	switch c {
	case protocol.ControlResign:
		return s.eliminate(g, st.user, game.ReasonResign)
	case protocol.ControlOfferDraw:
		s.drawAgreed = append(slices.Clip(s.drawAgreed), st.user)
		return s.imposeAgreedDraw(g), nil
	case protocol.ControlDeclineDraw:
		s.drawAgreed = nil
	case protocol.ControlAbort:
		s.imposed = &game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	}
	return s, nil
}

func (s roomState) imposeAgreedDraw(g Setup) roomState {
	if !s.finished() && s.everyActive(g, func(u game.UserID) bool { return slices.Contains(s.drawAgreed, u) }) {
		s.imposed = &game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
	}
	return s
}
