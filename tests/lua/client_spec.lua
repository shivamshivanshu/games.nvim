local Client = require("games.core.client")
local helpers = require("helpers")

local stub_conn = helpers.stub_conn

---@param seq integer
---@return games.GameUpdate
local function update(seq)
  return helpers.chess_update({ game_id = "g1", seq = seq })
end

local T = {}

function T.seq_gap_snapshots_instead_of_applying()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  conn:push("game.update", update(1))
  conn:push("game.update", update(3))

  assert(client:game("g1").seq == 1)
  assert(
    #conn.requests == 1 and conn.requests[1].type == "game.snapshot",
    vim.inspect(conn.requests)
  )
  assert(conn.requests[1].body.game_id == "g1")
  conn.requests[1].cb(nil, update(3))
  assert(client:game("g1").seq == 3)
end

function T.stale_error_snapshots_and_supersedes_the_reply()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local replies = {}
  local function record(err, result)
    table.insert(replies, { err = err and err.code or "none", result = result or "none" })
  end
  local move = { game_id = "g1", seq = 0, action = { kind = "move", move = "e4" } }
  client:game_act(move, record)
  client:game_act(move, record)
  conn.requests[1].cb({ code = "stale", message = "seq 0 is stale" }, nil)
  conn.requests[2].cb({ code = "illegal_action", message = "no" }, nil)

  assert(
    vim.deep_equal(replies, {
      { err = "none", result = "none" },
      { err = "illegal_action", result = "none" },
    }),
    vim.inspect(replies)
  )
  assert(conn.requests[3].type == "game.snapshot" and conn.requests[3].body.game_id == "g1")
end

function T.finished_games_beyond_the_cache_size_are_evicted_after_their_handlers_ran()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local handled = {}
  client:on_update(function(stored)
    handled[#handled + 1] = client:game(stored.game_id) ~= nil
  end)
  conn:push("game.update", helpers.chess_update({ game_id = "ongoing" }))
  for i = 1, 33 do
    conn:push(
      "game.update",
      helpers.chess_update({
        game_id = ("done%02d"):format(i),
        seq = 1,
        result = { kind = "draw", winners = {}, reason = "draw_agreed" },
      })
    )
  end

  assert(
    vim.iter(handled):all(function(cached)
      return cached
    end),
    "a handler ran after its update was evicted"
  )
  assert(client:game("done01") == nil, "the oldest finished game is still cached")
  assert(client:game("done02") and client:game("done33") and client:game("ongoing"))
end

---@param game_id games.GameID
---@param finished_at? integer
---@return games.GameUpdate
local function finished(game_id, finished_at)
  return helpers.chess_update({
    game_id = game_id,
    seq = 1,
    result = { kind = "draw", winners = {}, reason = "draw_agreed" },
    finished_at = finished_at,
  })
end

---@param client games.Client
---@param conn table
---@param games games.GameUpdate[]
local function list_finished(client, conn, games)
  client:game_list({ status = "finished" })
  conn.requests[#conn.requests].cb(nil, { games = games })
end

---@param client games.Client
---@param ids games.GameID[]
---@return games.GameID[]
local function cached(client, ids)
  return vim.tbl_filter(function(id)
    return client:game(id) ~= nil
  end, ids)
end

function T.the_cache_keeps_the_newest_finished_games_whatever_order_they_arrive_in()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local listed, ids = {}, {}
  for i = 1, 40 do
    local n = i * 7 % 40 + 1
    ids[n] = ("done%02d"):format(n)
    listed[i] = finished(ids[n], 1000 + n)
  end
  list_finished(client, conn, listed)

  helpers.assert_eq(vim.list_slice(ids, 9, 40), cached(client, ids))
end

function T.finished_ties_evict_the_lower_game_id_and_a_missing_finished_at_counts_as_oldest()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local ids = { "undated" }
  for i = 1, 32 do
    ids[i + 1] = ("done%02d"):format(i)
  end
  ids[34] = "done99"
  local listed = {}
  for i = 33, 2, -1 do
    listed[#listed + 1] = finished(ids[i], i <= 3 and 500 or 1000 + i)
  end
  listed[#listed + 1] = finished("undated")
  list_finished(client, conn, listed)
  helpers.assert_eq(vim.list_slice(ids, 2, 33), cached(client, ids))

  list_finished(client, conn, { finished("done99", 500) })
  helpers.assert_eq(vim.list_slice(ids, 3, 34), cached(client, ids))
end

function T.connected_resyncs_games_challenges_and_the_catalog()
  local conn = stub_conn()
  Client.new(conn --[[@as games.Conn]])
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })

  local types = vim.tbl_map(function(request)
    return request.type
  end, conn.requests)
  assert(
    vim.deep_equal(types, { "game.list", "challenge.list", "catalog.list" }),
    vim.inspect(types)
  )
end

function T.catalog_is_requested_once_per_connection()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local catalog = { games = { { id = "chess", name = "Chess" } } }
  client:catalog_list(nil)
  conn:requests_of("catalog.list")[1].cb({ code = "internal", message = "try again" }, nil)
  client:catalog_list(nil)
  assert(#conn:requests_of("catalog.list") == 2, "a failed catalog request must not be cached")
  conn:requests_of("catalog.list")[2].cb(nil, catalog)
  local infos
  client:game_infos(function(result)
    infos = result
  end)
  vim.wait(100, function()
    return infos ~= nil
  end)

  assert(vim.deep_equal(infos, { chess = catalog.games[1] }), vim.inspect(infos))
  assert(client:game_info("chess") == catalog.games[1])
  assert(#conn:requests_of("catalog.list") == 2)
  conn:push("disconnected", { code = "disconnected", message = "lost", retrying = true })
  client:catalog_list(nil)
  assert(#conn:requests_of("catalog.list") == 3)
  conn:requests_of("catalog.list")[3].cb(nil, catalog)
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })
  assert(#conn:requests_of("catalog.list") == 4, "a new connection refetches the catalog")
  assert(client:game_info("chess") == nil)
end

function T.catalog_indexes_each_game_s_actions_by_kind_until_the_connection_drops()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local move = { kind = "move", description = "a move", input = { field = "san", values = {} } }
  client:catalog_list(nil)
  conn:respond("catalog.list", {
    games = { { id = "chess", name = "Chess", actions = { move } }, { id = "go", name = "Go" } },
  })

  assert(vim.deep_equal(client:action_specs("chess"), { move = move }))
  assert(vim.deep_equal(client:action_specs("go"), {}))
  assert(client:action_specs("sudoku") == nil)
  conn:push("disconnected", { code = "disconnected", message = "lost", retrying = true })
  assert(client:action_specs("chess") == nil)
end

function T.game_infos_reads_the_catalog_it_was_delivered_even_when_the_connection_drops_first()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  client:catalog_list(nil)
  conn:respond("catalog.list", { games = { { id = "chess", name = "Chess" } } })
  local infos
  client:game_infos(function(by_type)
    infos = by_type
  end)
  conn:push("disconnected", { code = "disconnected", message = "lost", retrying = true })
  helpers.wait_for(function()
    return infos ~= nil
  end, "the catalog infos")
  assert(vim.deep_equal(vim.tbl_keys(infos), { "chess" }), vim.inspect(infos))
end

function T.resolved_challenge_ignores_late_pending()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local seen = {}
  client:on_challenge(function(challenge)
    table.insert(seen, challenge.state)
  end)
  local pending = {
    challenge_id = "c1",
    from = "alice",
    to = { "bob" },
    accepted = {},
    game = "chess",
    options = {},
    state = "pending",
    expires_at = 1792584300000,
  }
  local declined = vim.tbl_extend("force", pending, { state = "declined", by = "bob" })

  conn:push("challenge.changed", declined)
  conn:push("challenge.changed", pending)
  client:challenge_list(nil)
  conn.requests[#conn.requests].cb(nil, { challenges = { pending } })

  assert(vim.deep_equal(seen, { "declined" }), vim.inspect(seen))
  assert(#client:pending_challenges() == 0, vim.inspect(client:pending_challenges()))
end

function T.resolved_challenges_are_forgotten_at_the_next_login()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local pending = {
    challenge_id = "c1",
    from = "alice",
    to = { "bob" },
    accepted = {},
    game = "chess",
    options = {},
    state = "pending",
    expires_at = 1792584300000,
  }

  conn:push(
    "challenge.changed",
    vim.tbl_extend("force", pending, { state = "declined", by = "bob" })
  )
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })
  conn:push("challenge.changed", pending)

  helpers.assert_eq(
    { "c1" },
    vim.tbl_map(function(challenge)
      return challenge.challenge_id
    end, client:pending_challenges())
  )
end

function T.a_failing_update_handler_does_not_skip_the_others()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local seen = {}
  client:on_update(function()
    error("boom", 0)
  end)
  client:on_update(function(stored)
    table.insert(seen, stored.seq)
  end)
  local notified = helpers.capture_notifications(function()
    conn:push("game.update", update(1))
  end)

  helpers.assert_eq({ 1 }, seen)
  helpers.assert_eq({ "games: handling update for game g1 failed: boom" }, notified)
end

return T
