package room

import (
	"encoding/json"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// step is a step as the room applies it; only persistence sees its JSON form.
type step struct {
	seq     game.Seq
	user    game.UserID
	kind    store.StepKind
	action  json.RawMessage
	control protocol.Control
	at      time.Time
}

// paces reports whether st is a tick or a pause, which set a real-time game's
// pace and come too often for a player to have seen before acting.
func (st step) paces() bool { return st.kind == store.StepTick || st.kind == store.StepPause }

func (st step) record(remaining map[game.UserID]time.Duration) (store.StepRecord, error) {
	rec := store.StepRecord{Seq: st.seq, User: st.user, Kind: st.kind, Payload: st.action, At: st.at, ClockRemaining: remaining}
	if st.kind != store.StepControl {
		return rec, nil
	}
	payload, err := json.Marshal(st.control)
	if err != nil {
		return store.StepRecord{}, err
	}
	rec.Payload = payload
	return rec, nil
}

func stepOf(rec store.StepRecord) (step, error) {
	st := step{seq: rec.Seq, user: rec.User, kind: rec.Kind, at: rec.At}
	switch rec.Kind {
	case store.StepAction:
		st.action = rec.Payload
	case store.StepControl:
		if err := json.Unmarshal(rec.Payload, &st.control); err != nil {
			return step{}, protocol.BadRequestf("malformed control: %v", err)
		}
	}
	return st, nil
}
