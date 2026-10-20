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

---@param rendered games.Rendered
---@param row integer 0-based
---@return games.Hl[] the highlights on row, in column order
local function highlights_on(rendered, row)
  local marks = vim.tbl_filter(function(mark)
    return mark.row == row
  end, rendered.extmarks)
  table.sort(marks, function(a, b)
    return a.col < b.col
  end)
  return vim.tbl_map(function(mark)
    return mark.hl
  end, marks)
end

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
    hub.respond(conn, "game.list", { games = { hub.FINISHED } })
    vim.wait(20)
    fn(client, conn)
  end, debug.traceback)
  dashboard.close()
  assert(ok, err)
end

local T = {}

function T.renders_each_section_with_its_items_as_the_golden()
  local client = hub.client()
  local view = dashboard.view(hub.snapshot(client), false)
  assert_eq(GOLDEN_LINES, view.rendered.lines)
  assert_eq({ "GamesDashSection" }, highlights_on(view.rendered, 0))
  assert_eq({ "GamesWon" }, highlights_on(view.rendered, 11))
  assert_eq({ "GamesKey" }, highlights_on(view.rendered, 14))
  local without_recent = dashboard.view(hub.snapshot(client, { finished = {} }), false).rendered
  assert_eq("  none", without_recent.lines[12])
  assert_eq({ "GamesMuted" }, highlights_on(without_recent, 11))
  assert_eq(hub.INCOMING, view.targets[2].item)
  assert_eq(hub.YOUR_MOVE, view.targets[6].item)
  assert_eq("Sudoku", view.starts[2].item.name)
end

---@param client games.Client
---@return integer[] how many handlers follow each kind of client event
local function handler_counts(client)
  return { #client.update_handlers, #client.challenge_handlers, #client.catalog_handlers }
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
    assert_eq({ challenge_id = "c-in" }, hub.sent(conn, "challenge.accept"))
    cursor_to(3)
    press("d")
    assert_eq({ challenge_id = "c-out" }, hub.sent(conn, "challenge.cancel"))
    cursor_to(9)
    helpers.with_overrides({
      {
        vim.fn,
        "confirm",
        function()
          return 1
        end,
      },
    }, function()
      press("r")
    end)
    assert_eq(
      { game_id = hub.WAITING.game_id, seq = hub.WAITING.seq, control = "resign" },
      hub.sent(conn, "game.control")
    )
    press("2")
    assert_eq({ game = "sudoku", options = {} }, hub.sent(conn, "game.start"))
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
      assert(hub.requested(conn, "game.list"), "the dashboard did not fetch recent games")
    end)
    dashboard.close()
    assert(ok, err)
  end)
end

return T
