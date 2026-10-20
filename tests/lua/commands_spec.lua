local Archive = require("games.core.archive")
local Client = require("games.core.client")
local commands = require("games.core.commands")
local games = require("games")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local types = require("games.types")

local assert_eq = helpers.assert_eq
local MessageType = types.MessageType
local ErrorCode = types.ErrorCode

local LOGIN = { user = "alice", heartbeat_ms = 15000 }

---@class games.test.PromptStubs
---@field username? string what vim.ui.input answers
---@field password string what inputsecret answers
---@field asked string[] prompts shown, in order
---@field later? fun(answer: string|nil)[] when set, vim.ui.input answers are queued here instead
---@field notified string[]

---The plugin's client is a stub-backed one; prompts and notify are stubbed and restored after fn.
---@param credentials games.Credentials|nil
---@param fn fun(conn: games.test.StubConn, prompts: games.test.PromptStubs)
local function with_stub_client(credentials, fn)
  local conn = helpers.stub_conn(credentials)
  ---@type games.test.PromptStubs
  local prompts = { password = "", asked = {}, notified = {} }
  helpers.with_overrides({
    {
      games,
      "client",
      Client.new(conn --[[@as games.Conn]]),
    },
    {
      vim.ui,
      "input",
      function(opts, on_confirm)
        table.insert(prompts.asked, opts.prompt)
        if prompts.later then
          table.insert(prompts.later, on_confirm)
        else
          on_confirm(prompts.username)
        end
      end,
    },
    {
      vim.fn,
      "inputsecret",
      function(prompt)
        table.insert(prompts.asked, prompt)
        return prompts.password
      end,
    },
    {
      vim,
      "notify",
      function(message)
        table.insert(prompts.notified, message)
      end,
    },
  }, function()
    local ok, err = xpcall(fn, debug.traceback, conn, prompts)
    conn:push(
      "disconnected",
      { code = ErrorCode.DISCONNECTED, message = "closed", retrying = false }
    )
    assert(ok, err)
  end)
end

---@param conn games.test.StubConn
---@return boolean
local function listed_ongoing(conn)
  return vim.iter(conn.requests):any(function(request)
    return request.type == MessageType.GAME_LIST and request.body ~= nil
  end)
end

---@param conn games.test.StubConn
---@return boolean
local function fetched_stats(conn)
  return vim.iter(conn.requests):any(function(request)
    return request.type == MessageType.STATS_GET
  end)
end

---@param id string
---@param min integer
---@param max integer
---@return games.GameInfo
local function game_info(id, min, max)
  return { id = id, name = id, players = { min = min, max = max }, capabilities = {}, options = {} }
end

local CATALOG = {
  games = { game_info("chess", 2, 2), game_info("party", 3, 4), game_info("sudoku", 1, 1) },
}

---Answers the newest request of message_type with result.
---@param conn games.test.StubConn
---@param message_type string
---@param result table
local function respond(conn, message_type, result)
  local request = vim.iter(conn.requests):rev():find(function(candidate)
    return candidate.type == message_type
  end)
  assert(request, "no " .. message_type .. " request")
  request.cb(nil, result)
end

---@class games.test.SelectPrompt
---@field prompt string
---@field labels string[]

---Stubs vim.ui.select to answer each prompt with the item labelled answers[i], recording the prompts.
---@param answers string[]
---@param fn fun(prompts: games.test.SelectPrompt[])
local function with_select_answers(answers, fn)
  local prompts = {}
  helpers.with_overrides({
    {
      vim.ui,
      "select",
      function(items, opts, on_choice)
        local labels = vim.tbl_map(opts.format_item, items)
        table.insert(prompts, { prompt = opts.prompt, labels = labels })
        local answer = answers[#prompts]
        if not answer then
          on_choice(nil)
          return
        end
        local index = vim.fn.index(labels, answer)
        assert(index >= 0, ("%q is not offered by %q"):format(answer, opts.prompt))
        on_choice(items[index + 1])
      end,
    },
  }, function()
    fn(prompts)
  end)
end

---@param conn games.test.StubConn
---@return games.ChallengeCreateRequest|nil
local function sent_challenge(conn)
  local request = vim.iter(conn.requests):find(function(candidate)
    return candidate.type == MessageType.CHALLENGE_CREATE
  end)
  return request and request.body
end

local T = {}

function T.challenge_with_three_invitees_sends_all_of_them()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    commands.run({ "challenge", "party", "bob", "carol", "dave" })
    respond(conn, MessageType.CATALOG_LIST, CATALOG)
    assert_eq(
      { to = { "bob", "carol", "dave" }, game = "party", options = {} },
      sent_challenge(conn)
    )
    local frame = vim.iter(conn.requests):find(function(request)
      return request.type == MessageType.CHALLENGE_CREATE
    end).frame
    assert(frame:find('"options":{}', 1, true), frame)
  end)
end

function T.picked_opponents_limit_the_games_offered()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    with_select_answers({ "bob", "carol", "done", "party" }, function(prompts)
      commands.run({ "challenge" })
      respond(conn, MessageType.CATALOG_LIST, CATALOG)
      respond(conn, MessageType.USER_LIST, {
        users = {
          { user = "alice", online = true },
          { user = "bob", online = false },
          { user = "carol", online = false },
          { user = "dave", online = true },
        },
      })
      assert_eq({
        { prompt = "Challenge whom?", labels = { "dave (online)", "bob", "carol" } },
        { prompt = "Add another opponent? (bob)", labels = { "done", "dave (online)", "carol" } },
        { prompt = "Add another opponent? (bob, carol)", labels = { "done", "dave (online)" } },
        { prompt = "Game:", labels = { "party" } },
      }, prompts)
      assert_eq({ "bob", "carol" }, sent_challenge(conn).to)
    end)
  end)
end

function T.opponent_picking_stops_at_the_games_maximum()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    with_select_answers({ "bob" }, function(prompts)
      commands.run({ "challenge", "chess" })
      respond(conn, MessageType.CATALOG_LIST, CATALOG)
      respond(conn, MessageType.USER_LIST, {
        users = { { user = "bob", online = false }, { user = "carol", online = false } },
      })
      assert_eq(1, #prompts)
      assert_eq({ "bob" }, sent_challenge(conn).to)
    end)
  end)
end

function T.done_is_offered_only_once_the_named_games_minimum_is_picked()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    with_select_answers({ "bob", "carol", "done" }, function(prompts)
      commands.run({ "challenge", "party" })
      respond(conn, MessageType.CATALOG_LIST, CATALOG)
      respond(conn, MessageType.USER_LIST, {
        users = {
          { user = "bob", online = false },
          { user = "carol", online = false },
          { user = "dave", online = false },
        },
      })
      assert_eq({
        { prompt = "Challenge whom?", labels = { "bob", "carol", "dave" } },
        { prompt = "Challenge whom else? (bob)", labels = { "carol", "dave" } },
        { prompt = "Add another opponent? (bob, carol)", labels = { "done", "dave" } },
      }, prompts)
      assert_eq({ "bob", "carol" }, sent_challenge(conn).to)
    end)
  end)
end

function T.challenge_reports_a_game_that_does_not_fit_the_invitees()
  with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
    conn:push("connected", LOGIN)
    commands.run({ "challenge", "chess", "bob", "carol" })
    respond(conn, MessageType.CATALOG_LIST, CATALOG)
    assert_eq(nil, sent_challenge(conn))
    assert_eq({ 'games: no 3-player game "chess"' }, prompts.notified)
  end)
end

---@type games.GameInfo
local PUZZLE = {
  id = "puzzle",
  name = "Puzzle",
  players = { min = 1, max = 1 },
  capabilities = { "resign" },
  options = {
    level = { type = "enum", values = { "easy", "hard" }, default = "easy" },
    note = { type = "text", values = {}, default = "", max_len = 8 },
  },
  actions = {},
}

local SOLO_CATALOG = { games = vim.list_extend({ PUZZLE }, CATALOG.games) }

---@class games.test.InputPrompt
---@field prompt string
---@field default string

---Stubs vim.ui.input to answer each prompt with answers[i], false cancelling, recording the prompts.
---@param answers (string|false)[]
---@param fn fun(prompts: games.test.InputPrompt[])
local function with_input_answers(answers, fn)
  local prompts = {}
  helpers.with_overrides({
    {
      vim.ui,
      "input",
      function(opts, on_confirm)
        table.insert(prompts, { prompt = opts.prompt, default = opts.default })
        on_confirm(answers[#prompts] or nil)
      end,
    },
  }, function()
    fn(prompts)
  end)
end

function T.solo_starts_the_game_with_the_chosen_options_and_opens_the_reply()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    local opened = {}
    helpers.with_overrides({
      {
        shell,
        "open",
        function(_, game_id)
          table.insert(opened, game_id)
        end,
      },
    }, function()
      with_select_answers({ "hard" }, function()
        with_input_answers({ "r1c1" }, function()
          commands.run({ "solo", "puzzle" })
          respond(conn, MessageType.CATALOG_LIST, SOLO_CATALOG)
        end)
      end)
      assert_eq(
        { game = "puzzle", options = { level = "hard", note = "r1c1" } },
        vim.iter(conn.requests):rev():find(function(request)
          return request.type == MessageType.GAME_START
        end).body
      )
      respond(
        conn,
        MessageType.GAME_START,
        helpers.chess_update({
          game_id = "5010a1e0",
          game = "puzzle",
          players = { { user = "alice", seat = "solo" } },
        })
      )
    end)
    assert_eq({ "5010a1e0" }, opened)
  end)
end

function T.solo_without_a_game_offers_only_the_solo_games()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    with_select_answers({}, function(prompts)
      commands.run({ "solo" })
      respond(conn, MessageType.CATALOG_LIST, SOLO_CATALOG)
      assert_eq({ { prompt = "Solo game:", labels = { "Puzzle", "sudoku" } } }, prompts)
    end)
  end)
end

function T.solo_refuses_a_game_for_more_players()
  with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
    conn:push("connected", LOGIN)
    commands.run({ "solo", "chess" })
    respond(conn, MessageType.CATALOG_LIST, SOLO_CATALOG)
    assert_eq(
      false,
      vim.iter(conn.requests):any(function(request)
        return request.type == MessageType.GAME_START
      end)
    )
    assert_eq(
      { "games: chess is not a solo game; challenge someone with :Games challenge chess" },
      prompts.notified
    )
  end)
end

function T.solo_completes_the_catalogues_solo_games()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    respond(conn, MessageType.CATALOG_LIST, SOLO_CATALOG)
    assert_eq({ "puzzle", "sudoku" }, commands.complete("", "Games solo ", 11))
    assert_eq({ "sudoku" }, commands.complete("s", "Games solo s", 12))
  end)
end

function T.text_options_are_typed_within_their_limits_and_cancel_aborts()
  local cases = {
    {
      name = "a value fits",
      answers = { "r1c1" },
      defaults = { "" },
      sent = "r1c1",
      notified = {},
    },
    { name = "cancelling", answers = { false }, defaults = { "" }, notified = {} },
    {
      name = "over max_len",
      answers = { "123456789", "12345678" },
      defaults = { "", "123456789" },
      sent = "12345678",
      notified = { "games: note takes at most 8 bytes, not 9" },
    },
    {
      name = "a control character",
      answers = { "a\tb", "ab" },
      defaults = { "", "a\tb" },
      sent = "ab",
      notified = { "games: note cannot hold control characters" },
    },
  }
  for _, case in ipairs(cases) do
    with_stub_client({ username = "alice", password = "pw" }, function(conn, notices)
      conn:push("connected", LOGIN)
      with_select_answers({ "easy (default)" }, function()
        with_input_answers(case.answers, function(prompts)
          commands.run({ "solo", "puzzle" })
          respond(conn, MessageType.CATALOG_LIST, SOLO_CATALOG)
          assert_eq(
            vim.tbl_map(function(default)
              return { prompt = "note (up to 8 bytes): ", default = default }
            end, case.defaults),
            prompts
          )
        end)
      end)
      local start = vim.iter(conn.requests):find(function(request)
        return request.type == MessageType.GAME_START
      end)
      assert_eq(case.sent, start and start.body.options.note)
      assert_eq(case.notified, notices.notified)
    end)
  end
end

function T.connected_runs_immediately()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    commands.run({ "list" })
    assert(listed_ongoing(conn))
    assert_eq(0, conn.connects)
  end)
end

function T.missing_credentials_are_prompted_then_the_command_runs_once_connected()
  with_stub_client(nil, function(conn, prompts)
    prompts.username, prompts.password = "alice", "alice-pass"
    commands.run({ "list" })
    assert_eq({ "games username: ", "games password: " }, prompts.asked)
    assert_eq({ username = "alice", password = "alice-pass" }, conn.credentials)
    assert_eq(1, conn.connects)
    assert(not listed_ongoing(conn))

    conn:push("connected", LOGIN)
    assert(listed_ongoing(conn))
  end)
end

function T.only_missing_credentials_are_prompted()
  with_stub_client({ password = "configured" }, function(conn, prompts)
    prompts.username = "alice"
    commands.run({ "list" })
    assert_eq({ "games username: " }, prompts.asked)
    assert_eq({ username = "alice", password = "configured" }, conn.credentials)
  end)
end

function T.cancelled_or_empty_prompt_aborts()
  with_stub_client(nil, function(conn, prompts)
    commands.run({ "list" })
    assert_eq({ "games username: " }, prompts.asked)

    prompts.username = "alice"
    commands.run({ "list" })
    assert_eq({}, conn.credentials)
    assert_eq(0, conn.connects)
  end)
end

function T.a_command_during_an_open_prompt_replaces_the_pending_one_without_prompting()
  with_stub_client(nil, function(conn, prompts)
    prompts.later, prompts.password = {}, "alice-pass"
    commands.run({ "list" })
    commands.run({ "stats" })
    assert_eq(1, #prompts.later)
    assert_eq(
      { "games: still connecting — replaced pending `:Games list` with `:Games stats`" },
      prompts.notified
    )

    prompts.later[1]("alice")
    assert_eq(1, conn.connects)
    conn:push("connected", LOGIN)
    assert(fetched_stats(conn) and not listed_ongoing(conn))
  end)
end

function T.a_prompt_answered_after_connecting_leaves_nothing_waiting()
  with_stub_client({ password = "configured" }, function(conn, prompts)
    prompts.later = {}
    commands.run({ "list" })
    conn:push("connected", LOGIN)
    assert(listed_ongoing(conn))

    prompts.later[1]("alice")
    assert_eq(0, conn.connects)
  end)
end

function T.only_the_newest_pending_command_runs()
  with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
    commands.run({ "list" })
    commands.run({ "stats" })
    assert_eq(1, #prompts.notified)
    conn:push("connected", LOGIN)
    assert(fetched_stats(conn) and not listed_ongoing(conn))
  end)
end

function T.transient_disconnect_keeps_the_command_and_a_fatal_one_drops_it()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    commands.run({ "list" })
    conn:push("disconnected", { code = ErrorCode.DISCONNECTED, message = "x", retrying = true })
    conn:push("connected", LOGIN)
    assert(listed_ongoing(conn))

    conn.requests = {}
    conn:push("disconnected", { code = ErrorCode.DISCONNECTED, message = "x", retrying = true })
    commands.run({ "list" })
    conn:push("disconnected", { code = ErrorCode.AUTH_FAILED, message = "bad", retrying = false })
    conn:push("connected", LOGIN)
    assert(not listed_ongoing(conn))
  end)
end

local SEEN_LIVE = "5ee17e00-0000-4000-8000-000000000001"
local NEVER_SEEN = "5ee17e00-0000-4000-8000-000000000002"
local ARCHIVED_BEFORE = "5ee17e00-0000-4000-8000-000000000003"

---@param game_id string
---@param seq integer
---@return games.GameUpdate without a view
local function chess_game(game_id, seq)
  local update = helpers.chess_update({
    game_id = game_id,
    seq = seq,
    to_act = {},
    controls = {},
    started_at = 1792584000000,
  })
  update.view = nil
  return update
end

---@param game_id string
---@return games.GameUpdate as game.list returns it: no view
local function listed_finished(game_id)
  local update = chess_game(game_id, 2)
  update.result = { kind = "draw", winners = {}, reason = "draw_agreed" }
  return helpers.seal(update, ('{"game_id":"%s"}'):format(game_id))
end

---@param archive games.Archive
---@param update games.GameUpdate
local function archived(archive, update)
  local done = false
  archive:record(update, function()
    done = true
  end)
  helpers.wait_for(function()
    return done
  end, "the archive write")
end

function T.export_reports_written_and_present_games()
  with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
    helpers.with_temp_dir(function(root)
      local archive = Archive.attach(games.client, root)
      helpers.with_overrides({ { games, "archive", archive } }, function()
        conn:push("connected", LOGIN)
        archived(archive, chess_game(SEEN_LIVE, 1))
        archived(archive, listed_finished(ARCHIVED_BEFORE))

        commands.run({ "export" })
        respond(conn, MessageType.GAME_LIST, {
          games = {
            listed_finished(SEEN_LIVE),
            listed_finished(NEVER_SEEN),
            listed_finished(ARCHIVED_BEFORE),
          },
        })
        helpers.wait_for(function()
          return #prompts.notified > 0
        end, "the export report")
        assert_eq({ "games: export: 2 written, 1 already present" }, prompts.notified)
      end)
    end)
  end)
end

return T
