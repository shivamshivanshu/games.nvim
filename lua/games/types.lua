---@alias games.UserID string
---@alias games.GameID string
---@alias games.GameType string
---@alias games.Seat string
---@alias games.ChallengeID string
---@alias games.Capability "resign"|"draw"|"abort"|"clock"
---@alias games.ResultReason "resign"|"timeout"|"draw_agreed"|"aborted"|string
---@alias games.Options table<string, string>
---@alias games.ReplyCallback fun(err: games.ErrorBody|nil, result: table|nil)

---@class games.Envelope
---@field id? integer
---@field type games.MessageType
---@field body table

---@class games.ErrorBody
---@field code games.ErrorCode
---@field message string

---@class games.LoginRequest
---@field v integer
---@field username games.UserID
---@field password string

---@class games.LoginResult
---@field user games.UserID
---@field heartbeat_ms integer

---@class games.PlayerRange
---@field min integer
---@field max integer

---@class games.GameOption
---@field type "enum"
---@field values string[]
---@field default string

---@class games.GameInfo
---@field id games.GameType
---@field name string
---@field players games.PlayerRange
---@field capabilities games.Capability[]
---@field options table<string, games.GameOption>

---@class games.CatalogResult
---@field games games.GameInfo[]

---@class games.UserPresence
---@field user games.UserID
---@field online boolean

---@class games.UserListResult
---@field users games.UserPresence[]

---@class games.ChallengeCreateRequest
---@field to games.UserID[]
---@field game games.GameType
---@field options games.Options

---@class games.ChallengeCreateResult
---@field challenge_id games.ChallengeID

---@class games.ChallengeRef
---@field challenge_id games.ChallengeID

---@class games.Challenge
---@field challenge_id games.ChallengeID
---@field from games.UserID
---@field to games.UserID[]
---@field accepted games.UserID[]
---@field game games.GameType
---@field options games.Options
---@field state games.ChallengeState
---@field by? games.UserID
---@field game_id? games.GameID
---@field expires_at string

---@class games.ChallengeListResult
---@field challenges games.Challenge[]

---@class games.GameListRequest
---@field status? games.GameStatus

---@class games.GameListResult
---@field games games.GameUpdate[]

---@class games.GameSnapshotRequest
---@field game_id games.GameID

---@class games.Action
---@field kind string
---@field [string] any

---@class games.GameActRequest
---@field game_id games.GameID
---@field seq integer
---@field action games.Action

---@class games.GameControlRequest
---@field game_id games.GameID
---@field seq integer
---@field control games.Control

---@class games.SeqResult
---@field seq integer

---@class games.PlayerSeat
---@field user games.UserID
---@field seat games.Seat

---@class games.WireResult
---@field winners games.UserID[]
---@field reason games.ResultReason

---@class games.WireClock
---@field remaining_ms table<games.UserID, integer>
---@field running? games.UserID

---@class games.Event
---@field kind string
---@field [string] any

---@class games.GameUpdate
---@field game_id games.GameID
---@field game games.GameType
---@field options? games.Options
---@field seq integer
---@field players games.PlayerSeat[]
---@field to_act games.UserID[]
---@field result? games.WireResult
---@field clock? games.WireClock
---@field draw_agreed? games.UserID[]
---@field controls games.Control[]
---@field events games.Event[]
---@field view? table

---@class games.Tally
---@field wins integer
---@field losses integer
---@field draws integer

---@class games.StatsRecord: games.Tally
---@field user games.UserID
---@field game games.GameType

---@class games.HeadToHeadRecord: games.Tally
---@field user games.UserID
---@field opponent games.UserID
---@field game games.GameType

---@class games.StatsResult
---@field records games.StatsRecord[]
---@field head_to_head games.HeadToHeadRecord[]

local M = {}

M.PROTOCOL_VERSION = 1

---@enum games.MessageType
M.MessageType = {
  OK = "ok",
  ERROR = "error",
  AUTH_LOGIN = "auth.login",
  PING = "ping",
  CATALOG_LIST = "catalog.list",
  USER_LIST = "user.list",
  CHALLENGE_CREATE = "challenge.create",
  CHALLENGE_ACCEPT = "challenge.accept",
  CHALLENGE_DECLINE = "challenge.decline",
  CHALLENGE_CANCEL = "challenge.cancel",
  CHALLENGE_LIST = "challenge.list",
  GAME_LIST = "game.list",
  GAME_SNAPSHOT = "game.snapshot",
  GAME_ACT = "game.act",
  GAME_CONTROL = "game.control",
  STATS_GET = "stats.get",
  CHALLENGE_CHANGED = "challenge.changed",
  GAME_UPDATE = "game.update",
}

---@enum games.ErrorCode
M.ErrorCode = {
  BAD_REQUEST = "bad_request",
  VERSION_MISMATCH = "version_mismatch",
  UNAUTHENTICATED = "unauthenticated",
  AUTH_FAILED = "auth_failed",
  NOT_FOUND = "not_found",
  NOT_YOUR_TURN = "not_your_turn",
  ILLEGAL_ACTION = "illegal_action",
  STALE = "stale",
  INTERNAL = "internal",
  -- Client-side only: the server never sends it; the conn reports lost connections with it.
  DISCONNECTED = "disconnected",
}

---@enum games.ChallengeState
M.ChallengeState = {
  PENDING = "pending",
  ACCEPTED = "accepted",
  DECLINED = "declined",
  CANCELLED = "cancelled",
  EXPIRED = "expired",
}

---@enum games.GameStatus
M.GameStatus = {
  ONGOING = "ongoing",
  FINISHED = "finished",
}

---@enum games.Control
M.Control = {
  RESIGN = "resign",
  OFFER_DRAW = "offer_draw",
  DECLINE_DRAW = "decline_draw",
  ABORT = "abort",
}

return M
