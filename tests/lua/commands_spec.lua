local Archive = require("games.core.archive")
local Client = require("games.core.client")
local commands = require("games.core.commands")
local games = require("games")
local games_config = require("games.config")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local types = require("games.types")

local assert_eq = helpers.assert_eq
local MessageType = types.MessageType
local ErrorCode = types.ErrorCode
local Capability = types.Capability

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
    { games, "config", games_config.resolve({}) },
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
  }, function()
    local ok, err
    helpers.capture_notifications(function(notified)
      prompts.notified = notified
      ok, err = xpcall(fn, debug.traceback, conn, prompts)
    end)
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

local T = {}

function T.challenge_with_three_invitees_sends_all_of_them()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    commands.run({ "challenge", "party", "bob", "carol", "dave" })
    conn:respond(MessageType.CATALOG_LIST, CATALOG)
    assert_eq(
      { to = { "bob", "carol", "dave" }, game = "party", options = {} },
      conn:sent(MessageType.CHALLENGE_CREATE)
    )
    local frame = conn:requests_of(MessageType.CHALLENGE_CREATE)[1].frame
    assert(frame:find('"options":{}', 1, true), frame)
  end)
end

function T.picked_opponents_limit_the_games_offered()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    helpers.with_select_answers({ "bob", "carol", "done", "party" }, function(prompts)
      commands.run({ "challenge" })
      conn:respond(MessageType.CATALOG_LIST, CATALOG)
      conn:respond(MessageType.USER_LIST, {
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
      assert_eq({ "bob", "carol" }, conn:sent(MessageType.CHALLENGE_CREATE).to)
    end)
  end)
end

function T.opponent_picking_stops_at_the_games_maximum()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    helpers.with_select_answers({ "bob" }, function(prompts)
      commands.run({ "challenge", "chess" })
      conn:respond(MessageType.CATALOG_LIST, CATALOG)
      conn:respond(MessageType.USER_LIST, {
        users = { { user = "bob", online = false }, { user = "carol", online = false } },
      })
      assert_eq(1, #prompts)
      assert_eq({ "bob" }, conn:sent(MessageType.CHALLENGE_CREATE).to)
    end)
  end)
end

function T.done_is_offered_only_once_the_named_games_minimum_is_picked()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    helpers.with_select_answers({ "bob", "carol", "done" }, function(prompts)
      commands.run({ "challenge", "party" })
      conn:respond(MessageType.CATALOG_LIST, CATALOG)
      conn:respond(MessageType.USER_LIST, {
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
      assert_eq({ "bob", "carol" }, conn:sent(MessageType.CHALLENGE_CREATE).to)
    end)
  end)
end

function T.challenge_reports_a_game_that_does_not_fit_the_invitees()
  with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
    conn:push("connected", LOGIN)
    commands.run({ "challenge", "chess", "bob", "carol" })
    conn:respond(MessageType.CATALOG_LIST, CATALOG)
    assert_eq(nil, conn:sent(MessageType.CHALLENGE_CREATE))
    assert_eq({ 'games: no 3-player game "chess"' }, prompts.notified)
  end)
end

---@type games.GameInfo
local PUZZLE = {
  id = "puzzle",
  name = "Puzzle",
  players = { min = 1, max = 1 },
  capabilities = { Capability.RESIGN },
  options = {
    level = { type = "enum", values = { "easy", "hard" }, default = "easy" },
    note = { type = "text", values = {}, default = "", max_len = 8 },
  },
  actions = {},
}

local SOLO_CATALOG = {
  games = vim.list_extend({ PUZZLE }, vim.list_extend({ game_info("dice", 1, 4) }, CATALOG.games)),
}

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
      helpers.with_select_answers({ "hard" }, function()
        helpers.with_input_answers({ "r1c1" }, function()
          commands.run({ "solo", "puzzle" })
          conn:respond(MessageType.CATALOG_LIST, SOLO_CATALOG)
        end)
      end)
      assert_eq(
        { game = "puzzle", options = { level = "hard", note = "r1c1" } },
        conn:sent(MessageType.GAME_START)
      )
      conn:respond(
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
    helpers.with_select_answers({}, function(prompts)
      commands.run({ "solo" })
      conn:respond(MessageType.CATALOG_LIST, SOLO_CATALOG)
      assert_eq({ { prompt = "Solo game:", labels = { "Puzzle", "dice", "sudoku" } } }, prompts)
    end)
  end)
end

function T.solo_starts_a_game_that_also_takes_opponents()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    commands.run({ "solo", "dice" })
    conn:respond(MessageType.CATALOG_LIST, SOLO_CATALOG)
    assert_eq({ game = "dice", options = {} }, conn:sent(MessageType.GAME_START))
  end)
end

function T.solo_refuses_a_game_for_more_players()
  with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
    conn:push("connected", LOGIN)
    commands.run({ "solo", "chess" })
    conn:respond(MessageType.CATALOG_LIST, SOLO_CATALOG)
    assert_eq(false, conn:requested(MessageType.GAME_START))
    assert_eq(
      { "games: chess is not a solo game; challenge someone with :Games challenge chess" },
      prompts.notified
    )
  end)
end

function T.solo_reports_an_unknown_game_or_a_catalogue_without_solo_games()
  local multiplayer_only = {
    games = vim.tbl_filter(function(info)
      return info.players.max > 1
    end, CATALOG.games),
  }
  local cases = {
    { args = { "solo", "nope" }, catalog = SOLO_CATALOG, notified = 'games: no game "nope"' },
    { args = { "solo" }, catalog = multiplayer_only, notified = "games: no solo games available" },
  }
  for _, case in ipairs(cases) do
    with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
      conn:push("connected", LOGIN)
      helpers.with_select_answers({}, function(selects)
        commands.run(case.args)
        conn:respond(MessageType.CATALOG_LIST, case.catalog)
        assert_eq({}, selects)
      end)
      assert_eq({ case.notified }, prompts.notified)
      assert_eq(false, conn:requested(MessageType.GAME_START))
    end)
  end
end

function T.solo_completes_the_catalogues_solo_games()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    conn:respond(MessageType.CATALOG_LIST, SOLO_CATALOG)
    assert_eq({ "puzzle", "dice", "sudoku" }, commands.complete("", "Games solo ", 11))
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
      helpers.with_select_answers({ "easy (default)" }, function()
        helpers.with_input_answers(case.answers, function(prompts)
          commands.run({ "solo", "puzzle" })
          conn:respond(MessageType.CATALOG_LIST, SOLO_CATALOG)
          assert_eq(
            vim.tbl_map(function(default)
              return { prompt = "note (up to 8 bytes): ", default = default }
            end, case.defaults),
            prompts
          )
        end)
      end)
      local start = conn:sent(MessageType.GAME_START)
      assert_eq(case.sent, start and start.options.note)
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
    assert(conn:requested(MessageType.STATS_GET) and not listed_ongoing(conn))
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
    assert(conn:requested(MessageType.STATS_GET) and not listed_ongoing(conn))
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
        conn:respond(MessageType.GAME_LIST, {
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

---@param challenge_id string
---@param from games.UserID
---@param to games.UserID
---@return games.Challenge
local function pending_challenge(challenge_id, from, to)
  return {
    challenge_id = challenge_id,
    from = from,
    to = { to },
    accepted = {},
    game = "chess",
    options = {},
    state = "pending",
    expires_at = 1792584000000,
  }
end

function T.challenges_opens_the_requests_finder_once_the_challenges_and_catalog_are_in()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    helpers.with_select_answers({ "bob → you: Chess", "accept" }, function(prompts)
      commands.run({ "challenges" })
      assert_eq({}, prompts)
      conn:respond(MessageType.CHALLENGE_LIST, {
        challenges = {
          pending_challenge("c-out", "alice", "carol"),
          pending_challenge("c-in", "bob", "alice"),
        },
      })
      assert_eq({}, prompts)
      conn:respond(
        MessageType.CATALOG_LIST,
        { games = { vim.tbl_extend("force", game_info("chess", 2, 2), { name = "Chess" }) } }
      )
      assert_eq({
        { prompt = "Challenges:", labels = { "bob → you: Chess", "you → carol: Chess" } },
        { prompt = "bob → you: Chess:", labels = { "accept", "decline" } },
      }, prompts)
    end)
    assert_eq({ challenge_id = "c-in" }, conn:sent(MessageType.CHALLENGE_ACCEPT))
  end)
end

function T.challenges_and_list_report_what_they_cannot_show()
  local cases = {
    {
      args = { "challenges" },
      replies = {
        { MessageType.CHALLENGE_LIST, { challenges = {} } },
        { MessageType.CATALOG_LIST, CATALOG },
      },
      notified = "games: no challenges",
    },
    {
      args = { "list", "finished" },
      replies = { { MessageType.GAME_LIST, { games = {} } }, { MessageType.CATALOG_LIST, CATALOG } },
      notified = "games: no games",
    },
    {
      args = { "list", "paused" },
      notified = 'games: list takes ongoing or finished, not "paused"',
    },
  }
  for _, case in ipairs(cases) do
    with_stub_client({ username = "alice", password = "pw" }, function(conn, prompts)
      conn:push("connected", LOGIN)
      helpers.with_select_answers({}, function(selects)
        commands.run(case.args)
        for _, reply in ipairs(case.replies or {}) do
          conn:respond(unpack(reply))
        end
        assert_eq({}, selects)
      end)
      assert_eq({ case.notified }, prompts.notified)
    end)
  end
end

function T.list_opens_the_games_finder_over_the_games_of_that_status_alone()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    conn:push("game.update", chess_game(SEEN_LIVE, 1))
    local finished = listed_finished(NEVER_SEEN)
    local line = require("games.core.summary").format_game(finished, "alice", nil, 0)
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
      helpers.with_select_answers({ line }, function(prompts)
        commands.run({ "list", "finished" })
        assert_eq({ status = "finished" }, conn:sent(MessageType.GAME_LIST))
        conn:respond(MessageType.GAME_LIST, { games = { finished } })
        assert_eq({}, prompts)
        conn:respond(MessageType.CATALOG_LIST, CATALOG)
        assert_eq({ { prompt = "Games:", labels = { line } } }, prompts)
      end)
    end)
    assert_eq({ NEVER_SEEN }, opened)
  end)
end

---@type games.PuzzleEntry[]
local PUZZLE_ENTRIES = {
  {
    id = "SCHOL",
    mode = "puzzle",
    fen = "r1bqkbnr/pppp1ppp/2n5/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 3 3",
    moves = "g8f6 h5f7",
    player = "white",
  },
  { id = "game 1", mode = "study", fen = "", moves = "e4 e5 Ke3", player = "black" },
}

---Runs fn with the puzzle sets in a temp dir holding set "mixed", the latest, at its first entry.
---@param fn fun(dir: string)
local function with_puzzle_set(fn)
  local puzzles = require("games.core.puzzles")
  helpers.with_temp_dir(function(dir)
    vim.fn.mkdir(dir, "p")
    vim.fn.writefile(
      { vim.json.encode({ entries = PUZZLE_ENTRIES, next = 1 }) },
      dir .. "/mixed.json"
    )
    vim.fn.writefile({ "mixed" }, dir .. "/latest")
    helpers.with_overrides({
      {
        puzzles,
        "dir",
        function()
          return dir
        end,
      },
    }, function()
      fn(dir)
    end)
  end)
end

---@param dir string
---@return integer
local function next_puzzle(dir)
  return vim.json.decode(table.concat(vim.fn.readfile(dir .. "/mixed.json"), "\n")).next
end

---@param conn games.test.StubConn
---@return integer
local function game_starts(conn)
  return #conn:requests_of(MessageType.GAME_START)
end

function T.puzzle_starts_each_entry_of_the_set_in_turn_skipping_one_the_server_refuses()
  with_puzzle_set(function(dir)
    with_stub_client({ username = "alice", password = "pw" }, function(conn, notices)
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
        commands.run({ "puzzle" })
        helpers.wait_for(function()
          return game_starts(conn) == 1
        end, "the first puzzle's game.start")
        local first = PUZZLE_ENTRIES[1]
        assert_eq({
          game = "chess_puzzle",
          options = {
            mode = first.mode,
            fen = first.fen,
            moves = first.moves,
            player = first.player,
          },
        }, conn.requests[#conn.requests].body)
        conn:respond(
          MessageType.GAME_START,
          helpers.chess_update({ game_id = "p1", game = "chess_puzzle" })
        )
        helpers.wait_for(function()
          return next_puzzle(dir) == 2
        end, "the set to move past its first entry")
        assert_eq({ "p1" }, opened)

        commands.run({ "puzzle", "mixed" })
        helpers.wait_for(function()
          return game_starts(conn) == 2
        end, "the second puzzle's game.start")
        assert_eq("e4 e5 Ke3", conn.requests[#conn.requests].body.options.moves)
        conn.requests[#conn.requests].cb(
          { code = ErrorCode.BAD_REQUEST, message = "ply 3: illegal move: Ke3" },
          nil
        )
        helpers.wait_for(function()
          return next_puzzle(dir) == 3
        end, "the set to move past the refused entry")
        assert_eq({ "p1" }, opened)

        commands.run({ "puzzle" })
        helpers.wait_for(function()
          return #notices.notified == 3
        end, "the done notice")
      end)
      assert_eq({
        "games: puzzle mixed 1/2 (SCHOL)",
        "games: skipped mixed 2/2 (game 1): ply 3: illegal move: Ke3",
        "games: puzzle set mixed is done, all 2 played; restart it from :Games find puzzles",
      }, notices.notified)
      assert_eq(2, game_starts(conn))
    end)
  end)
end

function T.puzzle_completes_the_saved_sets_and_import_completes_files()
  with_puzzle_set(function()
    assert_eq({ "mixed" }, commands.complete("", "Games puzzle ", 13))
    assert_eq({ "mixed" }, commands.complete("m", "Games puzzle m", 14))
  end)
  local fixtures = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h")
    .. "/fixtures/puzzles/"
  local lead = fixtures .. "li"
  assert_eq(
    { fixtures .. "lichess.csv" },
    commands.complete(lead, "Games import " .. lead, 13 + #lead)
  )
end

function T.import_needs_no_server()
  with_puzzle_set(function(dir)
    with_stub_client(nil, function(conn, notices)
      local csv = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h")
        .. "/fixtures/puzzles/lichess.csv"
      commands.run({ "import", csv, "daily" })
      helpers.wait_for(function()
        return #notices.notified == 1
      end, "the import notice")
      assert_eq(0, conn.connects)
      assert_eq({}, notices.asked)
      assert(
        vim.startswith(notices.notified[1], "games: imported 4 into puzzle set daily, 2 failed:")
      )
      assert_eq({ "daily" }, vim.fn.readfile(dir .. "/latest"))
    end)
  end)
end

function T.puzzle_keeps_its_place_when_the_start_fails_short_of_a_refusal()
  with_puzzle_set(function(dir)
    with_stub_client({ username = "alice", password = "pw" }, function(conn, notices)
      conn:push("connected", LOGIN)
      for _, err in ipairs({
        { code = ErrorCode.DISCONNECTED, message = "connection lost" },
        { code = ErrorCode.INTERNAL, message = "internal error" },
      }) do
        local starts = game_starts(conn)
        commands.run({ "puzzle" })
        helpers.wait_for(function()
          return game_starts(conn) == starts + 1
        end, "the puzzle's game.start")
        conn.requests[#conn.requests].cb(err, nil)
        helpers.wait_for(function()
          return #notices.notified == starts + 1
        end, "the error notice")
        assert_eq(1, next_puzzle(dir))
      end
      assert_eq({ "games: connection lost", "games: internal error" }, notices.notified)
    end)
  end)
end

function T.a_set_imported_again_while_a_puzzle_starts_keeps_its_new_entries()
  with_puzzle_set(function(dir)
    with_stub_client({ username = "alice", password = "pw" }, function(conn, notices)
      conn:push("connected", LOGIN)
      helpers.with_overrides({ { shell, "open", function() end } }, function()
        commands.run({ "puzzle" })
        helpers.wait_for(function()
          return game_starts(conn) == 1
        end, "the puzzle's game.start")
        local reimported = { entries = { PUZZLE_ENTRIES[2], PUZZLE_ENTRIES[1] }, next = 1 }
        vim.fn.writefile({ vim.json.encode(reimported) }, dir .. "/mixed.json")
        conn:respond(
          MessageType.GAME_START,
          helpers.chess_update({ game_id = "p1", game = "chess_puzzle" })
        )
        helpers.wait_for(function()
          return #notices.notified == 1
        end, "the puzzle notice")
      end)
      assert_eq(
        { entries = { PUZZLE_ENTRIES[2], PUZZLE_ENTRIES[1] }, next = 1 },
        vim.json.decode(table.concat(vim.fn.readfile(dir .. "/mixed.json"), "\n"))
      )
    end)
  end)
end

function T.the_puzzles_finder_lists_the_saved_sets()
  with_puzzle_set(function()
    with_stub_client(nil, function()
      local opened
      helpers.with_overrides({
        {
          require("games.ui.finder"),
          "open",
          function(source)
            opened = source
          end,
        },
      }, function()
        require("games.core.commands.find").find(games.client, "alice", "puzzles")
        helpers.wait_for(function()
          return opened ~= nil
        end, "the finder")
      end)
      assert_eq({ { name = "mixed", entries = 2, next = 1 } }, opened.items)
    end)
  end)
end

function T.import_takes_a_max_over_the_configured_one()
  with_puzzle_set(function()
    with_stub_client(nil, function(_, notices)
      local csv = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h")
        .. "/fixtures/puzzles/lichess.csv"
      commands.run({ "import", csv, "daily", "none" })
      commands.run({ "import", csv, "daily", "2" })
      helpers.wait_for(function()
        return #notices.notified == 2
      end, "the import notices")
      assert_eq('games: import: max "none" is not a positive whole number', notices.notified[1])
      assert(
        vim.startswith(
          notices.notified[2],
          "games: imported 2 into puzzle set daily; kept 2, more entries left unread"
        ),
        notices.notified[2]
      )
    end)
  end)
end

return T
