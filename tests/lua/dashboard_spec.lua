local dashboard = require("games.ui.dashboard")
local helpers = require("helpers")
local hub = require("fixtures.hub")

local assert_eq = helpers.assert_eq

local GOLDEN_LINES = {
  "Requests",
  "  bob → you: Chess 10+5",
  "  you → carol: Chess none",
  "",
  "Your move",
  "  ● chess  vs bob     10+5  your move  4:51  last: bob played Nf3  #7f3a",
  "",
  "Ongoing",
  "    chess  vs carol   none  waiting          last: -  #8b2c",
  "",
  "Recent",
  "    chess  vs bob     none  finished         last: -  #9c4d",
  "",
  "New",
  "  1 Chess   2 players",
  "  2 Sudoku  solo",
  "",
  "<CR> open  / find  ? keys  q close",
}

---@param line integer 1-based
local function cursor_to(line)
  vim.api.nvim_win_set_cursor(0, { line, 0 })
end

---@param keys string
local function press(keys)
  vim.api.nvim_feedkeys(vim.keycode(keys), "x", false)
end

---@return string[]
local function shown_lines()
  return vim.api.nvim_buf_get_lines(0, 0, -1, false)
end

---Opens the dashboard over the hub client, answers its finished-games fetch, and closes it after fn.
---@param fn fun(client: games.Client, conn: games.test.StubConn)
local function with_dashboard(fn)
  local client, conn = hub.client()
  local ok, err = xpcall(function()
    dashboard.open(client)
    conn:respond("game.list", { games = { hub.FINISHED } })
    vim.wait(20)
    fn(client, conn)
  end, debug.traceback)
  dashboard.close()
  assert(ok, err)
end

---@type games.DashboardViewOptions
local CONNECTED_VIEW = { show_keys = false, connection = "connected" }

---@return integer timers running now, the dashboard's clock tick among them while it ticks
local function active_timers()
  local active = 0
  vim.uv.walk(function(handle)
    if handle:get_type() == "timer" and handle:is_active() then
      active = active + 1
    end
  end)
  return active
end

---@return boolean whether the dashboard's autocmd group exists
local function has_augroup()
  return (pcall(vim.api.nvim_get_autocmds, { group = "games.dashboard" }))
end

local T = {}

function T.renders_each_section_with_its_items_as_the_golden()
  local client = hub.client()
  local view = dashboard.view(hub.snapshot(client), CONNECTED_VIEW)
  assert_eq(GOLDEN_LINES, view.rendered.lines)
  assert_eq({ "GamesDashSection" }, helpers.hls_on_row(view.rendered, 0))
  assert_eq({ "GamesWon" }, helpers.hls_on_row(view.rendered, 11))
  assert_eq({ "GamesKey" }, helpers.hls_on_row(view.rendered, 14))
  local without_recent =
    dashboard.view(hub.snapshot(client, { finished = {} }), CONNECTED_VIEW).rendered
  assert_eq("  none", without_recent.lines[12])
  assert_eq({ "GamesMuted" }, helpers.hls_on_row(without_recent, 11))
  assert_eq(hub.INCOMING, view.targets[2].item)
  assert_eq(hub.YOUR_MOVE, view.targets[6].item)
  assert_eq("Sudoku", dashboard.start_target(view, 2).item.name)
end

---@param client games.Client
---@return integer[] how many handlers follow each kind of client event
local function handler_counts(client)
  return {
    #client.update_signal.handlers(),
    #client.challenge_signal.handlers(),
    #client.catalog_signal.handlers(),
    #client.connection_signal.handlers(),
  }
end

function T.updates_live_and_stops_following_the_client_once_closed()
  local client, conn = hub.client()
  local before = handler_counts(client)
  dashboard.open(client)
  local ok, err = pcall(function()
    local challenge = vim.tbl_extend(
      "force",
      hub.INCOMING,
      { challenge_id = "c-new", from = "dave", expires_at = hub.INCOMING.expires_at - 1 }
    )
    conn:push("challenge.changed", challenge)
    vim.wait(20)
    assert_eq("  dave → you: Chess 10+5", shown_lines()[2])
  end)
  local buf = vim.api.nvim_get_current_buf()
  dashboard.close()
  assert(ok, err)
  assert(not vim.api.nvim_buf_is_valid(buf), "the dashboard buffer outlived its window")
  assert_eq(before, handler_counts(client))
end

function T.keys_act_on_the_item_under_the_cursor()
  with_dashboard(function(_, conn)
    cursor_to(2)
    press("a")
    assert_eq({ challenge_id = "c-in" }, conn:sent("challenge.accept"))
    cursor_to(3)
    press("d")
    assert_eq({ challenge_id = "c-out" }, conn:sent("challenge.cancel"))
    cursor_to(9)
    helpers.confirming(1, function()
      press("r")
    end)
    assert_eq(
      { game_id = hub.WAITING.game_id, seq = hub.WAITING.seq, control = "resign" },
      conn:sent("game.control")
    )
    press("2")
    assert_eq({ game = "sudoku", options = {} }, conn:sent("game.start"))
  end)
end

function T.r_on_a_solo_game_gives_it_up()
  with_dashboard(function(_, conn)
    local alone = helpers.chess_update({
      game_id = "5010a1e0",
      game = "sudoku",
      players = { { user = "alice", seat = "solo" } },
      controls = { "resign" },
      view = {},
      started_at = hub.NOW_MS,
    })
    conn:push("game.update", alone)
    vim.wait(20)
    local line = vim.iter(ipairs(shown_lines())):find(function(_, shown)
      return shown:find("#5010", 1, true) ~= nil
    end)
    assert(line, vim.inspect(shown_lines()))
    assert(shown_lines()[line]:find("sudoku solo", 1, true), shown_lines()[line])
    cursor_to(line)
    local asked = helpers.confirming(1, function()
      press("r")
    end)
    assert_eq({ "Give up this game?" }, asked)
    assert_eq({ game_id = alone.game_id, seq = 0, control = "resign" }, conn:sent("game.control"))
  end)
end

function T.enter_opens_the_board_in_the_window_under_the_float_and_closes_the_dashboard()
  with_dashboard(function()
    local tiled = vim.fn.win_getid(vim.fn.winnr("#"))
    cursor_to(6)
    press("<CR>")
    vim.wait(20)
    assert(not dashboard.is_open(), "the dashboard stayed open over the board")
    assert_eq(tiled, vim.api.nvim_get_current_win())
    assert(vim.startswith(vim.api.nvim_buf_get_name(0), "games://chess/bob/"))
  end)
  vim.cmd("silent! bwipeout! games://chess/bob/7f3a")
end

function T.question_mark_lists_every_key()
  with_dashboard(function()
    press("?")
    local lines = shown_lines()
    assert(vim.tbl_contains(lines, "R     refresh"), vim.inspect(lines))
    assert(vim.tbl_contains(lines, "1-9   start the New game of that number"), vim.inspect(lines))
  end)
end

function T.games_without_arguments_opens_the_dashboard()
  local client, conn = hub.client()
  helpers.with_overrides({ { require("games"), "client", client } }, function()
    require("games.core.commands").run({})
    local ok, err = pcall(function()
      assert(dashboard.is_open(), "no dashboard")
      assert(conn:requested("game.list"), "the dashboard did not fetch recent games")
    end)
    dashboard.close()
    assert(ok, err)
  end)
end

function T.a_dropped_connection_pauses_the_clock_and_shows_reconnecting_until_it_is_back()
  with_dashboard(function(_, conn)
    local ticking = active_timers()
    conn:push("disconnected", { code = "disconnected", message = "lost", retrying = true })
    vim.wait(20)
    assert(dashboard.is_open(), "the dashboard closed on a dropped connection")
    assert_eq("reconnecting…", shown_lines()[1])
    assert_eq(ticking - 1, active_timers())

    conn.requests = {}
    conn:push("connected", { user = hub.ME, heartbeat_ms = 15000 })
    vim.wait(20)
    assert_eq("Requests", shown_lines()[1])
    assert_eq(ticking, active_timers())
    assert(conn:requested("game.list"), "recent games were not fetched again")
  end)
end

function T.a_burst_of_pushes_renders_once()
  with_dashboard(function(_, conn)
    local rendered, view = 0, dashboard.view
    helpers.with_overrides({
      {
        dashboard,
        "view",
        function(...)
          rendered = rendered + 1
          return view(...)
        end,
      },
    }, function()
      for seq = hub.WAITING.seq + 1, hub.WAITING.seq + 3 do
        conn:push("game.update", vim.tbl_extend("force", hub.WAITING, { seq = seq }))
      end
      vim.wait(20)
    end)
    assert_eq(1, rendered)
  end)
end

function T.close_drops_the_handlers_the_autocmds_and_the_clock_tick()
  local client = hub.client()
  local before, timers = handler_counts(client), active_timers()
  dashboard.open(client)
  assert_eq(timers + 1, active_timers())
  dashboard.close()
  assert_eq(before, handler_counts(client))
  assert(not has_augroup(), "the dashboard's autocmds outlived it")
  assert_eq(timers, active_timers())
end

function T.closing_its_window_any_other_way_cleans_up_as_close_does()
  local client = hub.client()
  local before = handler_counts(client)
  dashboard.open(client)
  vim.api.nvim_win_close(0, true)
  assert(not dashboard.is_open(), "the dashboard outlived its window")
  assert_eq(before, handler_counts(client))
  assert(not has_augroup(), "the dashboard's autocmds outlived its window")
end

function T.its_window_keeps_its_buffer_and_a_replaced_buffer_closes_it()
  local client, conn = hub.client()
  local before, timers = handler_counts(client), active_timers()
  dashboard.open(client)
  local win = vim.api.nvim_get_current_win()
  assert(vim.wo[win].winfixbuf, "winfixbuf is not set")
  vim.wo[win].winfixbuf = false
  local other = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_win_set_buf(win, other)
  assert_eq(before, handler_counts(client))
  assert_eq(timers, active_timers())
  conn:push("game.update", vim.tbl_extend("force", hub.YOUR_MOVE, { seq = hub.YOUR_MOVE.seq + 1 }))
  vim.wait(20)
  assert(not vim.api.nvim_win_is_valid(win), "the float stayed open over the replaced buffer")
  vim.api.nvim_buf_delete(other, { force = true })
end

function T.a_floating_picker_it_opens_leaves_it_open_when_cancelled()
  with_dashboard(function()
    helpers.with_overrides({
      {
        vim.ui,
        "select",
        function(_, _, on_choice)
          local buf = vim.api.nvim_create_buf(false, true)
          local float = vim.api.nvim_open_win(buf, true, {
            relative = "editor",
            width = 10,
            height = 2,
            row = 0,
            col = 0,
          })
          vim.wait(20)
          vim.api.nvim_win_close(float, true)
          vim.api.nvim_buf_delete(buf, { force = true })
          on_choice(nil)
        end,
      },
    }, function()
      cursor_to(2)
      press("<CR>")
      vim.wait(20)
    end)
    assert(dashboard.is_open(), "cancelling the dashboard's own picker closed it")
  end)
end

function T.a_key_acts_on_the_item_the_cursor_was_on_after_a_live_update_moves_it()
  with_dashboard(function(_, conn)
    cursor_to(9)
    local moved =
      vim.tbl_extend("force", hub.WAITING, { seq = hub.WAITING.seq + 1, to_act = { "alice" } })
    conn:push("game.update", moved)
    vim.wait(20)
    assert(
      shown_lines()[vim.api.nvim_win_get_cursor(0)[1]]:find("#8b2c", 1, true),
      "cursor lost the game"
    )
    helpers.confirming(1, function()
      press("r")
    end)
    assert_eq(
      { game_id = moved.game_id, seq = moved.seq, control = "resign" },
      conn:sent("game.control")
    )
  end)
end

return T
