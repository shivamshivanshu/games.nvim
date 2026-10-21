---@alias games.UserID string
---@alias games.GameID string
---@alias games.GameType string
---@alias games.Seat string
---@alias games.ChallengeID string
---@alias games.ActionKind string a game's own, listed in its catalog entry
---@alias games.Options table<string, string>
---Both nil means the request was superseded and the client fetches a fresh snapshot instead.
---@alias games.Reply<R> fun(err: games.ErrorBody|nil, result: R|nil)
---@alias games.ReplyCallback games.Reply<table>

---@class games.Envelope
---@field id? integer
---@field type games.MessageType
---@field body? table a request may omit it, meaning {}

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
---@field type games.OptionType
---@field values string[] empty for a text option
---@field default string
---@field max_len? integer bytes a text option takes, sent for text options only
---@field role? games.OptionRole set only on an option whose meaning the core knows

---The one text field an action takes from its player.
---@class games.ActionInput
---@field field string the action's field the text goes in
---@field description string
---@field pattern? string an RE2 expression the text matches, when values is empty
---@field values string[] the text is one of these, when not empty

---@class games.ActionSpec
---@field kind games.ActionKind
---@field description string
---@field input? games.ActionInput nil for an action that is its kind alone

---@class games.GameInfo
---@field id games.GameType
---@field name string
---@field players games.PlayerRange
---@field capabilities games.Capability[]
---@field options table<string, games.GameOption>
---@field actions games.ActionSpec[] the game's action catalog

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
---@field expires_at integer unix ms

---@class games.ChallengeListResult
---@field challenges games.Challenge[]

---Starts a solo game; the reply is its first games.GameUpdate.
---@class games.GameStartRequest
---@field game games.GameType
---@field options games.Options

---@class games.GameVerifyRequest
---@field summary string a finished game's summary bytes, as the server sent them

---@class games.GameListRequest
---@field status? games.GameStatus

---@class games.GameListResult
---@field games games.GameUpdate[]

---@class games.GameSnapshotRequest
---@field game_id games.GameID

---@class games.Action
---@field kind games.ActionKind
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
---@field kind games.ResultKind
---@field winners games.UserID[]
---@field reason string a games.ResultReason, or the game's own such as chess "checkmate"

---The clock as of as_of_ms: the step's time on a push, the read's on a snapshot.
---@class games.WireClock
---@field remaining_ms table<games.UserID, integer>
---@field running? games.UserID whose remaining time keeps falling from as_of_ms
---@field as_of_ms integer unix ms

---@alias games.EventKind string a game's own, such as chess "moved"

---@class games.Event
---@field kind games.EventKind
---@field [string] any

---@class games.GameUpdate
---@field game_id games.GameID
---@field game games.GameType
---@field options games.Options
---@field seq integer
---@field players games.PlayerSeat[]
---@field to_act games.UserID[]
---@field result? games.WireResult
---@field clock? games.WireClock
---@field draw_agreed games.UserID[]
---@field controls games.Control[]
---@field events games.Event[]
---@field view table the game's own view, as this player sees it; a finished game's final one
---@field started_at integer unix ms
---@field finished_at? integer unix ms
---@field summary? string the server's canonical summary bytes, once finished
---@field digest? string lowercase hex sha256 of summary
---@field exports? table<string, string> files the game records itself in, by name, once finished

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

---@class games.LeaderboardEntry: games.Tally
---@field user games.UserID

---@class games.StatsRequest
---@field user? games.UserID limits records and head_to_head to this user

---Ranked by the server: records by game, then rank; leaderboard by rank over every user and every
---game but those played alone.
---A rank orders most wins, then fewest losses, then most draws, then user.
---@class games.StatsResult
---@field records games.StatsRecord[]
---@field head_to_head games.HeadToHeadRecord[]
---@field leaderboard games.LeaderboardEntry[]

---@class games.ViewOptions per-buffer presentation choices
---@field glyphs games.Glyphs
---@field flipped boolean perspective toggled away from the player's default
---@field max_cell_height? integer
---@field art? games.ArtSet the game's asset pack in this glyph mode
---@field cursor? games.GridCursor where the game's cursor keys left the cursor; nil until moved
---@field grid_rows? games.GridRows the buffer's drawn grid rows, which its next draw reuses where they are unchanged

---@class games.LayoutCtx: games.ViewOptions
---@field me games.UserID
---@field space? games.Size what the board and its panels may fill; nil draws the smallest board
---@field ambiwidth games.Ambiwidth
---@field flip boolean whether a flippable game's board draws flipped, as the user now sees it

---The layout ctx of a module with asset_keys, whose pack render requires.
---@class games.ArtLayoutCtx: games.LayoutCtx
---@field art games.ArtSet

---@class games.DescribeCtx how event text outside a board draws
---@field art? games.ArtSet the game's asset pack in the user's glyph mode
---@field ambiwidth games.Ambiwidth

---A mark as the board draws it, and what it means, for the help.
---@class games.LegendEntry
---@field sample games.TextNode[]
---@field meaning string

---@class games.Display what the shell measured for one render
---@field text_area? games.Size the window space to fill; nil draws the smallest board
---@field ambiwidth? games.Ambiwidth the 'ambiwidth' in effect, default "single"
---@field seats? integer player lines the header reserves, held from the board's first update

---Prompts for the text of a catalogued action, which the shell sends in the field the catalogue
---names for it.
---@class games.GameInput
---@field kind games.ActionKind
---@field prompt string
---@field available? fun(update: games.GameUpdate): boolean whether it applies now; default always

---@class games.Flippable
---@field bottom_seat fun(update: games.GameUpdate): games.Seat|nil the seat whose player sees the board flipped; nil for none

---@class games.GameAction
---@field key string
---@field aliases? string[] more keys that do the same, mapped but left out of the footer, e.g. the arrows beside `hjkl`
---@field label string|fun(update: games.GameUpdate): string a function reads the label off the update, e.g. "call 40"
---@field action? games.Action
---@field input? games.GameInput prompts for text and sends it as its action, in place of action
---@field act? fun(update: games.GameUpdate, view: games.ViewOptions): games.Action|nil builds the action from the board's view options, such as its cursor, in place of action; nil binds nothing
---@field available? fun(update: games.GameUpdate): boolean whether the entry applies now; default always
---@field local_fn? fun(view: games.ViewOptions, update: games.GameUpdate): games.ViewOptions changes only the board's view options, which may follow the board's latest update

---@class games.GameModule
---@field layout fun(update: games.GameUpdate, ctx: games.LayoutCtx): games.Node
---@field input? games.GameInput
---@field actions? games.GameAction[]
---@field describers? table<games.EventKind, games.Describer> what text each event kind reads as; a kind without one, or one that returns nil, reads generically
---@field asset_keys? string[] keys every asset pack of the game must draw
---@field glyph_width? integer display columns of a one-line glyph, its cell_width at h = 1; default 1
---@field picture_keys? string[] keys its packs draw as whole pictures beside the board
---@field player_extra? fun(update: games.GameUpdate, seat: games.Seat, ctx: games.LayoutCtx): games.TextNode[]|nil what the header shows after the clock on seat's line
---@field flippable? games.Flippable the shell binds a key that turns the board, and sets ctx.flip
---@field legend fun(ctx: games.DescribeCtx): games.LegendEntry[] what each mark the board draws means, which the help shows

---@class games.OpenOptions
---@field glyphs games.Glyphs
---@field max_cell_height? integer
---@field assets? table<games.GameType, string> asset pack per game, default "default"

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
  GAME_START = "game.start",
  STATS_GET = "stats.get",
  GAME_VERIFY = "game.verify",
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
  SUMMARY_MISMATCH = "summary_mismatch",
  -- Client-side only: the server never sends it; the conn reports lost connections with it.
  DISCONNECTED = "disconnected",
}

---@enum games.Capability
M.Capability = {
  RESIGN = "resign",
  DRAW = "draw",
  ABORT = "abort",
  CLOCK = "clock",
}

---@enum games.ChallengeState
M.ChallengeState = {
  PENDING = "pending",
  ACCEPTED = "accepted",
  DECLINED = "declined",
  CANCELLED = "cancelled",
  EXPIRED = "expired",
}

---@enum games.ResultReason reasons every game shares
M.ResultReason = {
  RESIGN = "resign",
  TIMEOUT = "timeout",
  DRAW_AGREED = "draw_agreed",
  ABORTED = "aborted",
}

---The reason a game played alone ends once its player solves it, as fifteen and mastermind do.
M.SOLVED_REASON = "solved"

---@enum games.ResultKind
M.ResultKind = {
  DECISIVE = "decisive",
  DRAW = "draw",
  ABORTED = "aborted",
}

---@enum games.GameStatus
M.GameStatus = {
  ONGOING = "ongoing",
  FINISHED = "finished",
}

---@enum games.OptionType
M.OptionType = {
  ENUM = "enum",
  TEXT = "text",
}

---@enum games.OptionRole
M.OptionRole = {
  CLOCK = "clock",
}

---@enum games.Control
M.Control = {
  RESIGN = "resign",
  OFFER_DRAW = "offer_draw",
  DECLINE_DRAW = "decline_draw",
  ABORT = "abort",
}

---@enum games.Glyphs
M.Glyphs = { UNICODE = "unicode", ASCII = "ascii" }

---@enum games.Palette
M.Palette = { THEME = "theme", VIVID = "vivid" }

---The 'ambiwidth' a render measures under.
---@enum games.Ambiwidth
M.Ambiwidth = { SINGLE = "single", DOUBLE = "double" }

M.DEFAULT_AMBIWIDTH = M.Ambiwidth.SINGLE

---How a finished game went for one player.
---@enum games.Outcome
M.Outcome = { WON = "won", LOST = "lost", DRAWN = "drawn", ABORTED = "aborted" }

---@enum games.PickerName
M.PickerName = {
  SNACKS = "snacks",
  TELESCOPE = "telescope",
  FZF = "fzf",
  SELECT = "select",
}

return M
