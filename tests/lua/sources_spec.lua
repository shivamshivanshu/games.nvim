local helpers = require("helpers")
local hub = require("fixtures.hub")
local sources = require("games.core.sources")

local assert_eq = helpers.assert_eq
local SourceName = sources.SourceName

---@param source games.Source
---@return string[]
local function formatted(source)
  return vim.tbl_map(source.format, source.items)
end

---@param source games.Source
---@param item any
---@return string[]
local function applicable_names(source, item)
  return vim.tbl_map(function(action)
    return action.name
  end, sources.applicable(source, item))
end

---@param source games.Source
---@param name games.SourceActionName
---@param item any
local function run(source, name, item)
  assert(sources.find_action(source, item, { name }), name .. " does not apply").run(item)
end

local T = {}

function T.requests_list_incoming_first_and_offer_only_the_side_s_own_actions()
  local client = hub.client()
  local requests = sources.build(SourceName.REQUESTS, hub.snapshot(client))
  assert_eq({ "bob → you: Chess 10+5", "you → carol: Chess none" }, formatted(requests))
  assert_eq({ "accept", "decline" }, applicable_names(requests, hub.INCOMING))
  assert_eq({ "cancel" }, applicable_names(requests, hub.OUTGOING))
  assert_eq(nil, requests.default_action)
  assert_eq({
    "from     bob",
    "to       alice",
    "accepted -",
    "game     Chess 10+5",
    "options  time_control=10+5",
    "expires  in 5:00",
  }, requests.preview(hub.INCOMING))
end

function T.games_put_your_move_first_then_waiting_then_finished_newest_first()
  local client = hub.client()
  local older_finish = vim.tbl_extend("force", hub.FINISHED, {
    game_id = "0ld00000",
    finished_at = hub.FINISHED.finished_at - 1,
  })
  local snapshot = hub.snapshot(client, { finished = { older_finish, hub.FINISHED } })
  local games = sources.build(SourceName.GAMES, snapshot)
  assert_eq({ "7f3a91c2", "8b2c0e14", "9c4d5a77", "0ld00000" }, vim.tbl_map(games.key, games.items))
  assert_eq(
    "● chess  vs bob     10+5  your move  4:51  last: bob played Nf3  #7f3a",
    games.format(games.items[1])
  )
  assert_eq({ "open", "resign" }, applicable_names(games, hub.YOUR_MOVE))
  assert_eq({ "open" }, applicable_names(games, hub.FINISHED))
end

function T.a_listed_game_the_cache_holds_newer_is_shown_as_cached()
  local client, conn = hub.client()
  local moved =
    vim.tbl_extend("force", hub.WAITING, { seq = hub.WAITING.seq + 1, to_act = { "alice" } })
  conn:push("game.update", moved)
  local snapshot = hub.snapshot(client, { finished = { hub.WAITING } })
  local games = sources.build(SourceName.GAMES, snapshot)
  local shown = vim.iter(games.items):find(function(update)
    return update.game_id == hub.WAITING.game_id
  end)
  assert_eq(moved.seq, shown.seq)
  assert_eq(2, #games.items)
end

function T.game_preview_interpolates_the_running_clock_from_as_of_ms()
  local client = hub.client()
  local games = sources.build(SourceName.GAMES, hub.snapshot(client))
  local preview = games.preview(hub.YOUR_MOVE)
  assert(vim.tbl_contains(preview, "clock   alice 4:51, bob 4:48"), vim.inspect(preview))
  assert(vim.tbl_contains(preview, "status  your move"), vim.inspect(preview))
end

function T.types_come_from_the_catalog_with_its_action_grammar_in_the_preview()
  local client = hub.client()
  local game_types = sources.build(SourceName.TYPES, hub.snapshot(client))
  assert_eq({ "Chess   2 players", "Sudoku  solo" }, formatted(game_types))
  assert_eq({
    "game         Chess",
    "players      2 players",
    "controls     resign, draw, abort, clock",
    "time control 10+5, none (default 10+5)",
    "move         a move in SAN or UCI",
  }, game_types.preview(game_types.items[1]))
end

function T.players_leave_out_me_and_rank_by_the_server_s_leaderboard()
  local client = hub.client()
  local snapshot = hub.snapshot(client, {
    users = {
      { user = "alice", online = true },
      { user = "carol", online = false },
      { user = "bob", online = true },
    },
    stats = {
      records = {
        { user = "carol", game = "chess", wins = 3, losses = 0, draws = 1 },
        { user = "bob", game = "chess", wins = 1, losses = 2, draws = 0 },
      },
      head_to_head = {
        { user = "bob", opponent = "carol", game = "chess", wins = 1, losses = 2, draws = 0 },
      },
      leaderboard = {
        { user = "carol", wins = 3, losses = 0, draws = 1 },
        { user = "bob", wins = 1, losses = 2, draws = 0 },
      },
    },
  })
  local players = sources.build(SourceName.PLAYERS, snapshot)
  assert_eq({ "bob (online)", "carol" }, formatted(players))
  assert_eq({
    "user             bob",
    "online           yes",
    "rank             2 of 2",
    "chess            W 1  L 2  D 0",
    "vs carol (chess) W 1  L 2  D 0",
  }, players.preview(players.items[1]))
end

function T.puzzles_list_nothing_until_puzzle_sets_exist()
  local client = hub.client()
  local puzzles = sources.build(SourceName.PUZZLES, hub.snapshot(client))
  assert_eq({}, puzzles.items)
  assert_eq({}, puzzles.actions)
end

function T.actions_send_through_the_client()
  local client, conn = hub.client()
  local snapshot = hub.snapshot(client)
  run(sources.build(SourceName.REQUESTS, snapshot), "accept", hub.INCOMING)
  assert_eq({ challenge_id = "c-in" }, hub.sent(conn, "challenge.accept"))
  run(sources.build(SourceName.REQUESTS, snapshot), "cancel", hub.OUTGOING)
  assert_eq({ challenge_id = "c-out" }, hub.sent(conn, "challenge.cancel"))

  helpers.with_overrides({
    {
      vim.fn,
      "confirm",
      function()
        return 1
      end,
    },
  }, function()
    run(sources.build(SourceName.GAMES, snapshot), "resign", hub.YOUR_MOVE)
  end)
  assert_eq({ game_id = "7f3a91c2", seq = 6, control = "resign" }, hub.sent(conn, "game.control"))
end

function T.starting_a_solo_type_starts_it_and_any_other_challenges()
  local client, conn = hub.client()
  local game_types = sources.build(SourceName.TYPES, hub.snapshot(client))
  local chess, sudoku = game_types.items[1], game_types.items[2]
  run(game_types, "start", sudoku)
  assert_eq({ game = "sudoku", options = {} }, hub.sent(conn, "game.start"))

  helpers.with_overrides({
    {
      vim.ui,
      "select",
      function(_, _, on_choice)
        on_choice(nil)
      end,
    },
  }, function()
    run(game_types, "start", chess)
    vim.wait(20)
    assert(hub.requested(conn, "user.list"), "challenging chess asks for an opponent")
  end)
end

---@param answer integer what vim.fn.confirm answers: 1 yes, 2 no
---@param fn fun()
local function confirming(answer, fn)
  helpers.with_overrides({
    {
      vim.fn,
      "confirm",
      function()
        return answer
      end,
    },
  }, fn)
end

function T.resign_sends_the_seq_the_client_holds_once_confirmed()
  local client, conn = hub.client()
  local games = sources.build(SourceName.GAMES, hub.snapshot(client))
  local built = hub.WAITING
  local moved = vim.tbl_extend("force", built, { seq = built.seq + 1 })
  confirming(1, function()
    conn:push("game.update", moved)
    run(games, "resign", built)
  end)
  assert_eq(
    { game_id = built.game_id, seq = moved.seq, control = "resign" },
    hub.sent(conn, "game.control")
  )
end

function T.resign_declined_sends_nothing()
  local client, conn = hub.client()
  local games = sources.build(SourceName.GAMES, hub.snapshot(client))
  confirming(2, function()
    run(games, "resign", hub.WAITING)
  end)
  assert_eq(false, hub.requested(conn, "game.control"))
end

return T
