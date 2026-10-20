package protocol

import "encoding/json"

const Version = 1

type MessageType string

const (
	TypeOK               MessageType = "ok"
	TypeError            MessageType = "error"
	TypeAuthLogin        MessageType = "auth.login"
	TypePing             MessageType = "ping"
	TypeCatalogList      MessageType = "catalog.list"
	TypeUserList         MessageType = "user.list"
	TypeChallengeCreate  MessageType = "challenge.create"
	TypeChallengeAccept  MessageType = "challenge.accept"
	TypeChallengeDecline MessageType = "challenge.decline"
	TypeChallengeCancel  MessageType = "challenge.cancel"
	TypeChallengeList    MessageType = "challenge.list"
	TypeGameList         MessageType = "game.list"
	TypeGameSnapshot     MessageType = "game.snapshot"
	TypeGameAct          MessageType = "game.act"
	TypeGameControl      MessageType = "game.control"
	TypeStatsGet         MessageType = "stats.get"
	TypeGameVerify       MessageType = "game.verify"
	TypeChallengeChanged MessageType = "challenge.changed"
	TypeGameUpdate       MessageType = "game.update"
)

type ErrorCode string

const (
	CodeBadRequest      ErrorCode = "bad_request"
	CodeVersionMismatch ErrorCode = "version_mismatch"
	CodeUnauthenticated ErrorCode = "unauthenticated"
	CodeAuthFailed      ErrorCode = "auth_failed"
	CodeNotFound        ErrorCode = "not_found"
	CodeNotYourTurn     ErrorCode = "not_your_turn"
	CodeIllegalAction   ErrorCode = "illegal_action"
	CodeStale           ErrorCode = "stale"
	CodeInternal        ErrorCode = "internal"
	CodeSummaryMismatch ErrorCode = "summary_mismatch"
)

// RequestID is chosen by the client and echoed on the reply.
type RequestID uint64

// Envelope.ID is nil on pushes.
type Envelope struct {
	ID   *RequestID      `json:"id,omitempty"`
	Type MessageType     `json:"type"`
	Body json.RawMessage `json:"body"`
}

type ErrorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}
