local Client = require("games.core.client")
local helpers = require("helpers")
local notify = require("games.core.notify")

local assert_eq = helpers.assert_eq

local AFTER_E4_FEN = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1"

---@return games.test.StubConn conn
---@return {[1]: string, [2]: integer}[] notified
---@return fun() restore
local function alice_with_notify()
  local conn = helpers.stub_conn()
  notify.attach(Client.new(conn --[[@as games.Conn]]))
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })
  local notified, original = {}, vim.notify
  vim.notify = function(message, level)
    table.insert(notified, { message, level })
  end
  return conn, notified, function()
    vim.notify = original
  end
end

---@param seq integer
---@param moved_by games.Seat|nil
---@param to_act games.UserID[]
---@return games.GameUpdate
local function chess_update(seq, moved_by, to_act)
  return helpers.chess_update({
    game_id = "notify01",
    seq = seq,
    to_act = to_act,
    controls = {},
    events = moved_by and { { kind = "moved", by = moved_by, san = "e4" } } or {},
    view = { fen = AFTER_E4_FEN, history_san = { "e4" }, legal_moves = {}, claimable_draw = false },
  })
end

---@param fn fun(conn: games.test.StubConn, notified: {[1]: string, [2]: integer}[])
local function with_alice(fn)
  local conn, notified, restore = alice_with_notify()
  local ok, err = xpcall(fn, debug.traceback, conn, notified)
  restore()
  assert(ok, err)
end

local T = {}

function T.own_moves_do_not_notify()
  with_alice(function(conn, notified)
    conn:push("game.update", chess_update(0, nil, { "alice" }))
    conn:push("game.update", chess_update(1, "white", { "bob" }))
    assert_eq({}, notified)

    conn:push("game.update", chess_update(2, "black", { "alice" }))
    assert_eq(
      { { "games: chess vs bob: bob played e4 — your move", vim.log.levels.INFO } },
      notified
    )
  end)
end

function T.giving_up_notifies_errors()
  with_alice(function(conn, notified)
    conn:push(
      "disconnected",
      { code = "disconnected", message = "connection lost", retrying = true }
    )
    conn:push("disconnected", { code = "disconnected", message = "closed", retrying = false })
    conn:push(
      "disconnected",
      { code = "auth_failed", message = "invalid credentials", retrying = false }
    )
    conn:push("disconnected", { code = "version_mismatch", message = "want v2", retrying = false })

    assert_eq(2, #notified)
    assert_eq(
      "games: login failed (invalid credentials); not reconnecting, check username and password, or run :Games register for a new account",
      notified[1][1]
    )
    assert(notified[2][1]:find("update the plugin", 1, true), notified[2][1])
    assert_eq({ vim.log.levels.ERROR, vim.log.levels.ERROR }, { notified[1][2], notified[2][2] })
  end)
end

function T.a_refused_sign_up_says_why_and_what_to_do()
  with_alice(function(conn, notified)
    for _, code in ipairs({ "signup_disabled", "passphrase_rejected", "user_exists" }) do
      conn:push("disconnected", { code = code, message = "the reason", retrying = false })
    end
    assert_eq({
      {
        "games: sign-up failed (the reason); ask the server's host for an account",
        vim.log.levels.ERROR,
      },
      {
        "games: sign-up failed (the reason); check the passphrase with the server's host",
        vim.log.levels.ERROR,
      },
      { "games: sign-up failed (the reason); pick another username", vim.log.levels.ERROR },
    }, notified)
  end)
end

function T.a_sign_up_refused_for_another_reason_still_says_it_failed()
  with_alice(function(conn, notified)
    local refused = { code = "bad_request", message = "password too long", retrying = false }
    conn:push("disconnected", refused)
    conn:push("disconnected", vim.tbl_extend("force", refused, { sign_up = true }))
    assert_eq({ { "games: sign-up failed (password too long)", vim.log.levels.ERROR } }, notified)
  end)
end

function T.resolution_messages_name_game_and_opponent()
  local challenge = {
    challenge_id = "c1",
    from = "alice",
    to = { "bob" },
    accepted = {},
    game = "chess",
    options = {},
    expires_at = 1792584300000,
  }
  local function text(state, by, me)
    local resolved = vim.tbl_extend("force", challenge, { state = state, by = by })
    local message = notify.resolution_message(resolved, me)
    return message and message("Chess 10+5")
  end

  assert_eq("bob declined your Chess 10+5 challenge", text("declined", "bob", "alice"))
  assert_eq(nil, text("declined", "bob", "bob"))
  assert_eq("your Chess 10+5 challenge to bob expired", text("expired", nil, "alice"))
  assert_eq("alice's Chess 10+5 challenge expired", text("expired", nil, "bob"))
  assert_eq("alice cancelled the Chess 10+5 challenge", text("cancelled", "alice", "bob"))
  assert_eq(nil, text("cancelled", "alice", "alice"))
end

return T
