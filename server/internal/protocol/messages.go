package protocol

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type Control string

const (
	ControlResign      Control = "resign"
	ControlOfferDraw   Control = "offer_draw"
	ControlDeclineDraw Control = "decline_draw"
	ControlAbort       Control = "abort"
)

type ChallengeState string

const (
	ChallengePending   ChallengeState = "pending"
	ChallengeAccepted  ChallengeState = "accepted"
	ChallengeDeclined  ChallengeState = "declined"
	ChallengeCancelled ChallengeState = "cancelled"
	ChallengeExpired   ChallengeState = "expired"
)

type GameStatus string

const (
	StatusOngoing  GameStatus = "ongoing"
	StatusFinished GameStatus = "finished"
)

type OptionType string

const OptionEnum OptionType = "enum"

type ChallengeID string

type Empty struct{}

type LoginRequest struct {
	V        int         `json:"v"`
	Username game.UserID `json:"username"`
	Password string      `json:"password"`
}

type LoginResult struct {
	User        game.UserID `json:"user"`
	HeartbeatMS int64       `json:"heartbeat_ms"`
}

type PlayersWire struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type OptionWire struct {
	Type    OptionType         `json:"type"`
	Values  []game.OptionValue `json:"values"`
	Default game.OptionValue   `json:"default"`
}

type GameInfoWire struct {
	ID           game.GameType                  `json:"id"`
	Name         string                         `json:"name"`
	Players      PlayersWire                    `json:"players"`
	Capabilities []game.Capability              `json:"capabilities"`
	Options      map[game.OptionName]OptionWire `json:"options"`
}

func InfoToWire(info game.Info) GameInfoWire {
	options := make(map[game.OptionName]OptionWire, len(info.Options))
	for _, spec := range info.Options {
		options[spec.Name] = OptionWire{Type: OptionEnum, Values: append([]game.OptionValue{}, spec.Values...), Default: spec.Default}
	}
	return GameInfoWire{
		ID:           info.Type,
		Name:         info.Name,
		Players:      PlayersWire{Min: info.Players.Min, Max: info.Players.Max},
		Capabilities: append([]game.Capability{}, info.Capabilities...),
		Options:      options,
	}
}

type CatalogResult struct {
	Games []GameInfoWire `json:"games"`
}

type UserPresence struct {
	User   game.UserID `json:"user"`
	Online bool        `json:"online"`
}

type UserListResult struct {
	Users []UserPresence `json:"users"`
}

type ChallengeCreateRequest struct {
	To      []game.UserID   `json:"to"`
	Game    game.GameType   `json:"game"`
	Options json.RawMessage `json:"options"`
}

type ChallengeCreateResult struct {
	ChallengeID ChallengeID `json:"challenge_id"`
}

type ChallengeRef struct {
	ChallengeID ChallengeID `json:"challenge_id"`
}

type Challenge struct {
	ChallengeID ChallengeID    `json:"challenge_id"`
	From        game.UserID    `json:"from"`
	To          []game.UserID  `json:"to"`
	Accepted    []game.UserID  `json:"accepted"`
	Game        game.GameType  `json:"game"`
	Options     game.Options   `json:"options"`
	State       ChallengeState `json:"state"`
	By          *game.UserID   `json:"by"`
	GameID      *game.GameID   `json:"game_id"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

// Participants lists the challenger first, then the invitees.
func (c Challenge) Participants() []game.UserID {
	return append([]game.UserID{c.From}, c.To...)
}

type ChallengeListResult struct {
	Challenges []Challenge `json:"challenges"`
}

type GameListRequest struct {
	Status GameStatus `json:"status,omitempty"`
}

type GameListResult struct {
	Games []GameUpdate `json:"games"`
}

type GameSnapshotRequest struct {
	GameID game.GameID `json:"game_id"`
}

type GameActRequest struct {
	GameID game.GameID     `json:"game_id"`
	Seq    game.Seq        `json:"seq"`
	Action json.RawMessage `json:"action"`
}

type GameControlRequest struct {
	GameID  game.GameID `json:"game_id"`
	Seq     game.Seq    `json:"seq"`
	Control Control     `json:"control"`
}

type SeqResult struct {
	Seq game.Seq `json:"seq"`
}

type ResultKind string

const (
	ResultDecisive ResultKind = "decisive"
	ResultDraw     ResultKind = "draw"
	ResultAborted  ResultKind = "aborted"
)

func ResultKindOf(k game.ResultKind) (ResultKind, error) {
	switch k {
	case game.ResultDecisive:
		return ResultDecisive, nil
	case game.ResultDraw:
		return ResultDraw, nil
	case game.ResultAborted:
		return ResultAborted, nil
	}
	return "", fmt.Errorf("protocol: unknown result kind %d", k)
}

type WireResult struct {
	Kind    ResultKind        `json:"kind"`
	Winners []game.UserID     `json:"winners"`
	Reason  game.ResultReason `json:"reason"`
}

type WireClock struct {
	RemainingMS map[game.UserID]int64 `json:"remaining_ms"`
	Running     *game.UserID          `json:"running"`
}

type GameUpdate struct {
	GameID     game.GameID       `json:"game_id"`
	Game       game.GameType     `json:"game"`
	Options    game.Options      `json:"options"`
	Seq        game.Seq          `json:"seq"`
	Players    []game.Player     `json:"players"`
	ToAct      []game.UserID     `json:"to_act"`
	Result     *WireResult       `json:"result"`
	Clock      *WireClock        `json:"clock"`
	DrawAgreed []game.UserID     `json:"draw_agreed"`
	Controls   []Control         `json:"controls"`
	Events     []json.RawMessage `json:"events"`
	View       json.RawMessage   `json:"view"`
}

type Tally struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
}

type StatsRecord struct {
	User game.UserID   `json:"user"`
	Game game.GameType `json:"game"`
	Tally
}

type HeadToHeadRecord struct {
	User     game.UserID   `json:"user"`
	Opponent game.UserID   `json:"opponent"`
	Game     game.GameType `json:"game"`
	Tally
}

type StatsResult struct {
	Records    []StatsRecord      `json:"records"`
	HeadToHead []HeadToHeadRecord `json:"head_to_head"`
}
