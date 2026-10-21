local Client = require("games.core.client")
local helpers = require("helpers")
local sources = require("games.core.sources")

---A logged-in alice with two challenges, a game on her move, one waiting on carol, a finished
---one, and a catalog of chess and sudoku: what the dashboard and the sources are built from.
local M = {}

M.ME = "alice"
M.NOW_MS = 1792584000000
local MINUTE_MS = 60000

---@type games.GameInfo[]
M.CATALOG = { helpers.game_info("sudoku"), helpers.game_info("chess") }

---@type games.Challenge
M.INCOMING = {
  challenge_id = "c-in",
  from = "bob",
  to = { "alice" },
  accepted = {},
  game = "chess",
  options = { time_control = "10+5" },
  state = "pending",
  expires_at = M.NOW_MS + 5 * MINUTE_MS,
}

---@type games.Challenge
M.OUTGOING = {
  challenge_id = "c-out",
  from = "alice",
  to = { "carol" },
  accepted = {},
  game = "chess",
  options = { time_control = "none" },
  state = "pending",
  expires_at = M.NOW_MS + MINUTE_MS,
}

M.YOUR_MOVE = helpers.chess_update({
  game_id = "7f3a91c2",
  seq = 6,
  options = { time_control = "10+5" },
  players = { { user = "alice", seat = "white" }, { user = "bob", seat = "black" } },
  clock = {
    remaining_ms = { alice = 300000, bob = 288000 },
    running = "alice",
    as_of_ms = M.NOW_MS - 9000,
  },
  events = { { kind = "moved", by = "black", san = "Nf3" } },
  started_at = M.NOW_MS - 10 * MINUTE_MS,
})

M.WAITING = helpers.chess_update({
  game_id = "8b2c0e14",
  seq = 3,
  options = { time_control = "none" },
  players = { { user = "carol", seat = "white" }, { user = "alice", seat = "black" } },
  to_act = { "carol" },
  controls = { "resign", "offer_draw" },
  started_at = M.NOW_MS - 30 * MINUTE_MS,
})

M.FINISHED = helpers.chess_update({
  game_id = "9c4d5a77",
  seq = 40,
  options = { time_control = "none" },
  to_act = {},
  controls = {},
  result = { kind = "decisive", winners = { "alice" }, reason = "checkmate" },
  started_at = M.NOW_MS - 120 * MINUTE_MS,
  finished_at = M.NOW_MS - 60 * MINUTE_MS,
})

---A client logged in as alice whose cache holds the catalog, both challenges and the ongoing
---games; the finished game is left for a game.list finished to return.
---@return games.Client
---@return games.test.StubConn
function M.client()
  local conn = helpers.stub_conn({ username = M.ME, password = helpers.PASSWORD })
  local client = Client.new(conn --[[@as games.Conn]])
  conn:push("connected", { user = M.ME, heartbeat_ms = 15000 })
  conn:respond("catalog.list", { games = M.CATALOG })
  conn:respond("challenge.list", { challenges = { M.OUTGOING, M.INCOMING } })
  conn:respond("game.list", { games = { M.WAITING, M.YOUR_MOVE } })
  return client, conn
end

---What client holds, with the finished game listed, read at NOW_MS.
---@param client games.Client
---@param extra? games.SourceExtra
---@return games.SourceSnapshot
function M.snapshot(client, extra)
  local snapshot = sources.snapshot(client, M.ME, extra or { finished = { M.FINISHED } })
  return vim.tbl_extend("force", snapshot, { now_ms = M.NOW_MS })
end

return M
