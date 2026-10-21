local Client = require("games.core.client")
local board_widget = require("games.ui.widgets.board")
local text_util = require("games.ui.text")
local paint = require("games.ui.paint")
local shell = require("games.ui.shell")
local chess = require("games.games.chess")
local helpers = require("helpers")
local timer = require("games.core.timer")

local assert_eq = helpers.assert_eq

local AFTER_E4_FEN = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1"
local FOOLS_MATE_FEN = "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3"
local ASCII = helpers.view_options("chess", chess, "ascii")
local UNICODE = helpers.view_options("chess", chess, "unicode")

local chess_update = helpers.chess_update

local function board_lines(lines)
  return vim.tbl_filter(function(line)
    return line:match("^%d ") ~= nil
  end, lines)
end

local function without_trailing_space(line)
  return (line:gsub("%s+$", ""))
end

local function hl_at_square(rendered, square)
  local file, rank = square:byte(1) - ("a"):byte() + 1, square:sub(2, 2)
  for i, line in ipairs(rendered.lines) do
    if vim.startswith(line, rank .. " ") then
      return helpers.hl_at(rendered, i - 1, 2 + (file - 1) * 2)
    end
  end
end

---The highlight of the extmark starting where needle first appears.
---@param rendered games.Rendered
---@param needle string
---@return games.Hl|nil
local function hl_of(rendered, needle)
  for i, line in ipairs(rendered.lines) do
    local start = line:find(needle, 1, true)
    if start then
      return helpers.hl_at(rendered, i - 1, start - 1)
    end
  end
  error(("%q not rendered"):format(needle))
end

---A real client on a stub conn, logged in as me (unless nil) with updates already pushed.
---@param me games.UserID|nil
---@param updates games.GameUpdate[]
---@return games.Client
---@return games.test.StubConn
local function client_with(me, updates)
  local conn = helpers.stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  if me then
    conn:push("connected", { user = me, heartbeat_ms = 15000 })
  end
  for _, update in ipairs(updates) do
    conn:push("game.update", update)
  end
  return client, conn
end

---@param conn games.test.StubConn
---@param message_type string
---@return table[] bodies of the requests of that type
local function sent(conn, message_type)
  return vim.tbl_map(function(request)
    return request.body
  end, conn:requests_of(message_type))
end

local function mapping(buf, lhs)
  for _, map in ipairs(vim.api.nvim_buf_get_keymap(buf, "n")) do
    if map.lhs == lhs then
      return map
    end
  end
end

local function first_board_line(buf)
  return board_lines(vim.api.nvim_buf_get_lines(buf, 0, -1, false))[1]
end

local T = {}

function T.render_start_position()
  local rendered = shell.render(chess_update({}), chess, ASCII, "alice")

  local header = rendered.lines[1]
  assert(header:find("alice", 1, true) and header:find("your move", 1, true), header)
  assert_eq("8 r n b q k b n r   K king", without_trailing_space(board_lines(rendered.lines)[1]))
  local footer = rendered.lines[#rendered.lines]
  assert(footer:find("R resign", 1, true) and footer:find("A abort", 1, true), footer)
  assert(not footer:find("X ", 1, true), footer)
end

function T.render_flipped_for_black()
  local rendered = shell.render(chess_update({}), chess, ASCII, "bob")

  assert(vim.startswith(board_lines(rendered.lines)[1], "1 "), vim.inspect(rendered.lines))
end

function T.last_move_and_check_marks()
  local update = chess_update({
    seq = 4,
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = { "bob" }, reason = "checkmate" },
    view = {
      fen = FOOLS_MATE_FEN,
      history_san = { "f3", "e5", "g4", "Qh4#" },
      last_move = { from = "d8", to = "h4" },
      legal_moves = {},
      claimable_draw = false,
      check_square = "e1",
    },
  })
  local rendered = shell.render(update, chess, ASCII, "alice")

  assert_eq(
    { "GamesSquareDark", "GamesLastMove", "GamesLastMoveDark", "GamesLastMoveDarkInk" },
    hl_at_square(rendered, "d8")
  )
  assert_eq(
    { "GamesSquareDark", "GamesLastMove", "GamesLastMoveDark", "GamesPieceBlack" },
    hl_at_square(rendered, "h4")
  )
  assert_eq({ "GamesSquareDark", "GamesCheck", "GamesPieceWhite" }, hl_at_square(rendered, "e1"))
  assert_eq("GamesLost", hl_of(rendered, "Result: bob won"))
  assert_eq({ "GamesLastMove", "GamesLastMoveInk", "GamesCheckSign" }, hl_of(rendered, "#"))
  assert_eq("GamesMuted", hl_of(rendered, "1."))
end

function T.last_move_on_a_light_square_layers_square_tint_then_piece()
  local update = chess_update({
    view = {
      fen = AFTER_E4_FEN,
      history_san = { "e4" },
      last_move = { from = "e2", to = "e4" },
      legal_moves = {},
      claimable_draw = false,
    },
  })
  local rendered = shell.render(update, chess, ASCII, "alice")

  assert_eq(
    { "GamesSquareLight", "GamesLastMove", "GamesPieceWhite" },
    hl_at_square(rendered, "e4")
  )
  assert_eq({ "GamesLastMove", "GamesLastMoveInk" }, hl_of(rendered, "e4"))
end

function T.header_names_take_seat_colours_and_footer_keys_their_own()
  local update = chess_update({
    clock = { remaining_ms = { alice = 9000, bob = 300000 }, running = "alice" },
  })
  local rendered = shell.render(update, chess, ASCII, "alice")

  assert_eq("GamesSeat1", hl_of(rendered, "alice"))
  assert_eq("GamesSeat2", hl_of(rendered, "bob"))
  assert_eq("GamesClockLow", hl_of(rendered, "0:09"))
  assert_eq(nil, hl_of(rendered, "5:00"))
  assert_eq("GamesYourTurn", hl_of(rendered, " your move "))
  assert_eq("GamesKey", hl_of(rendered, "R resign"))
  assert_eq("GamesLabel", hl_of(rendered, "resign"))
end

function T.player_extra_follows_the_clock_on_its_seats_line_only()
  local update = chess_update({
    clock = { remaining_ms = { alice = 300000, bob = 300000 }, running = "alice" },
  })
  local with_extra = vim.tbl_extend("force", chess, {
    player_extra = function(_, seat)
      return seat == "black" and { { kind = "text", str = "x", hl = "GamesMuted" } } or nil
    end,
  })
  local rendered, header_height = shell.render(update, with_extra, ASCII, "alice")

  assert_eq({
    "alice  white  5:00     ←  your move ",
    "bob    black  5:00  x               ",
  }, vim.list_slice(rendered.lines, 1, 2))
  assert_eq(3, header_height)
  assert_eq("GamesMuted", hl_of(rendered, "x "))
end

function T.unicode_board_shows_hollow_white_and_solid_black()
  local rows = board_lines(shell.render(chess_update({}), chess, UNICODE, "alice").lines)
  assert(vim.startswith(rows[1], "8 ♜ ♞ ♝ ♛ ♚ ♝ ♞ ♜"), rows[1])
  assert(vim.startswith(rows[8], "1 ♖ ♘ ♗ ♕ ♔ ♗ ♘ ♖"), rows[8])
end

function T.open_fetches_the_snapshot_of_a_game_the_client_does_not_hold()
  local client, conn = client_with("alice", {})
  assert_eq(nil, shell.open(client, "fetch01"))
  assert_eq({ { game_id = "fetch01" } }, sent(conn, "game.snapshot"))

  local finished = chess_update({
    game_id = "fetch01",
    seq = 4,
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = { "bob" }, reason = "checkmate" },
  })
  conn.requests[#conn.requests].cb(nil, finished)
  local buf = vim.api.nvim_get_current_buf()
  local name = vim.api.nvim_buf_get_name(buf)
  local text = table.concat(vim.api.nvim_buf_get_lines(buf, 0, -1, false), "\n")
  vim.api.nvim_buf_delete(buf, { force = true })

  assert_eq("games://chess/bob/fetc", name)
  assert(text:find("Result: bob won (checkmate)", 1, true), text)
  assert(text:find("\n8 ", 1, true), text)
end

---@param update games.GameUpdate
---@param catalogued? table<games.ActionKind, true> chess's whole catalogue when nil
---@return string
local function footer_of(update, catalogued)
  local rendered =
    shell.render(update, chess, ASCII, "alice", 0, nil, catalogued or helpers.CHESS_ACTIONS)
  return rendered.lines[#rendered.lines]
end

function T.claim_draw_is_offered_only_when_the_view_says_it_is_claimable()
  local claimable = chess_update({})
  claimable.view = vim.tbl_extend("force", claimable.view, { claimable_draw = true })

  assert(not footer_of(chess_update({})):find("C claim draw", 1, true))
  assert(footer_of(claimable):find("C claim draw", 1, true), footer_of(claimable))
end

function T.game_actions_the_catalog_does_not_list_are_not_bound()
  local claimable = chess_update({})
  claimable.view = vim.tbl_extend("force", claimable.view, { claimable_draw = true })
  local footer = footer_of(claimable, { claim_draw = true })

  assert(footer:find("C claim draw", 1, true) and not footer:find("m move", 1, true), footer)
  assert(footer:find("F flip", 1, true), footer)
end

function T.game_actions_stay_unbound_until_the_catalog_arrives_then_every_board_rebinds()
  local client, conn = client_with("alice", { chess_update({ game_id = "cat001" }) })
  local buf = shell.open(client, "cat001")
  assert(not mapping(buf, "m"), "move is bound before the catalog is known")
  assert(mapping(buf, "R"), "controls do not wait for the catalog")

  conn:respond("catalog.list", {
    games = {
      {
        id = "chess",
        name = "Chess",
        actions = { { kind = "move", input = { field = "move" } }, { kind = "claim_draw" } },
      },
    },
  })

  assert(mapping(buf, "m"), "move is not bound once the catalog arrives")
  assert(vim.api.nvim_buf_get_lines(buf, -2, -1, false)[1]:find("m move", 1, true))
  vim.api.nvim_buf_delete(buf, { force = true })
end

function T.an_action_with_input_prompts_and_sends_its_text_while_catalogued_and_available()
  ---@type games.GameModule
  local module = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    actions = {
      {
        key = "f",
        label = "flag",
        input = {
          kind = "flag",
          prompt = "Flag: ",
        },
        available = function(update)
          return update.view.flaggable
        end,
      },
    },
  }
  local function flag_binding(update, catalogued)
    local rendered, _, bindings = shell.render(update, module, ASCII, "alice", 0, nil, catalogued)
    local binding = vim.iter(bindings):find(function(b)
      return b.key == "f"
    end)
    assert_eq(binding ~= nil, rendered.lines[#rendered.lines]:find("f flag", 1, true) ~= nil)
    return binding
  end
  local flaggable = chess_update({ game_id = "flag01", seq = 4, view = { flaggable = true } })
  local catalogued = { flag = { kind = "flag", description = "flag", input = { field = "cell" } } }

  assert(not flag_binding(flaggable, { move = true }), "bound without the catalogue listing it")
  assert(not flag_binding(chess_update({ view = { flaggable = false } }), catalogued))
  assert(not flag_binding(
    chess_update({
      view = { flaggable = true },
      result = { kind = "aborted", winners = {}, reason = "aborted" },
    }),
    catalogued
  ), "bound on a finished game")
  local acted = {}
  local board = {
    update = flaggable,
    client = {
      game_act = function(_, request)
        acted[#acted + 1] = request
      end,
    },
  }
  helpers.with_input_answers({ "c5" }, function(prompts)
    flag_binding(flaggable, catalogued).run(board)
    assert_eq({ { prompt = "Flag: " } }, prompts)
  end)
  assert_eq({ { game_id = "flag01", seq = 4, action = { kind = "flag", cell = "c5" } } }, acted)
end

function T.running_clock_counts_down_from_arrival()
  local update = chess_update({
    clock = { remaining_ms = { alice = 300000, bob = 300000 }, running = "alice" },
  })
  local rendered = shell.render(update, chess, ASCII, "alice", 61000)

  assert(rendered.lines[1]:find("3:59", 1, true), rendered.lines[1])
  assert(rendered.lines[2]:find("5:00", 1, true), rendered.lines[2])
end

function T.controls_drive_keymaps()
  local client, conn = client_with("alice", { chess_update({ game_id = "keys01" }) })
  local buf = shell.open(client, "keys01")

  assert_eq("games://chess/bob/keys", vim.api.nvim_buf_get_name(buf))
  assert(mapping(buf, "R") and not mapping(buf, "X"))
  conn:push(
    "game.update",
    chess_update({
      game_id = "keys01",
      seq = 1,
      controls = { "resign", "offer_draw", "decline_draw" },
      draw_agreed = { "bob" },
    })
  )
  assert(mapping(buf, "X") and not mapping(buf, "A"))
end

function T.resign_confirms_then_sends_control()
  local client, conn = client_with("alice", { chess_update({ game_id = "resign01", seq = 3 }) })
  local buf = shell.open(client, "resign01")
  helpers.confirming(2, function()
    mapping(buf, "R").callback()
  end)
  assert_eq({}, sent(conn, "game.control"))
  helpers.confirming(1, function()
    mapping(buf, "R").callback()
  end)

  assert_eq({ { game_id = "resign01", seq = 3, control = "resign" } }, sent(conn, "game.control"))
end

function T.a_game_played_alone_is_named_solo_and_offers_to_give_up()
  local alone = chess_update({
    game_id = "solo0001",
    players = { { user = "alice", seat = "white" } },
    controls = { "resign" },
  })
  local client, conn = client_with("alice", { alone })
  local buf = shell.open(client, "solo0001")
  assert_eq("games://chess/solo/solo", vim.api.nvim_buf_get_name(buf))
  local footer = vim.api.nvim_buf_get_lines(buf, -2, -1, false)[1]
  assert(footer:find("R give up", 1, true) and not footer:find("resign", 1, true), footer)

  local asked = helpers.confirming(1, function()
    mapping(buf, "R").callback()
  end)
  assert_eq({ "Give up this game?" }, asked)
  assert_eq({ { game_id = "solo0001", seq = 0, control = "resign" } }, sent(conn, "game.control"))
end

function T.flip_toggles_black_default_perspective()
  local client = client_with("bob", { chess_update({ game_id = "flip01" }) })
  local buf = shell.open(client, "flip01")
  assert(vim.startswith(first_board_line(buf), "1 "), first_board_line(buf))

  mapping(buf, "F").callback()
  assert(vim.startswith(first_board_line(buf), "8 "), first_board_line(buf))
end

function T.open_charges_clock_from_its_as_of_time()
  local as_of_ms = 1792584000000
  local update = chess_update({
    game_id = "clock01",
    clock = {
      remaining_ms = { alice = 300000, bob = 300000 },
      running = "alice",
      as_of_ms = as_of_ms,
    },
  })
  local client = client_with("alice", { update })
  local header
  helpers.with_overrides({
    {
      timer,
      "unix_ms",
      function()
        return as_of_ms + 61000
      end,
    },
  }, function()
    local buf = shell.open(client, "clock01")
    header = vim.api.nvim_buf_get_lines(buf, 0, 1, false)[1]
    vim.api.nvim_buf_delete(buf, { force = true })
  end)

  assert(header:find("3:59", 1, true), header)
end

function T.open_refuses_while_disconnected()
  local client = client_with(nil, { chess_update({ game_id = "offline01" }) })
  local messages = helpers.capture_notifications(function()
    assert_eq(nil, shell.open(client, "offline01"))
  end)

  assert_eq({ "games: not connected" }, messages)
end

function T.reopen_after_bdelete_builds_fresh_board()
  local client = client_with("alice", { chess_update({ game_id = "bdel01" }) })
  local buf = shell.open(client, "bdel01")
  vim.cmd.bdelete(buf)
  vim.wait(100, function()
    return not vim.api.nvim_buf_is_valid(buf)
  end)
  assert(not vim.api.nvim_buf_is_valid(buf), "the deleted board buffer is wiped")

  local reopened = shell.open(client, "bdel01")
  assert_eq("games://chess/bob/bdel", vim.api.nvim_buf_get_name(reopened))
  assert(
    first_board_line(reopened),
    vim.inspect(vim.api.nvim_buf_get_lines(reopened, 0, -1, false))
  )
  assert(mapping(reopened, "R"))
end

function T.hidden_buffer_marked_dirty()
  local client, conn = client_with("alice", { chess_update({ game_id = "hide01" }) })
  local buf = shell.open(client, "hide01")
  vim.cmd.enew()

  conn:push(
    "game.update",
    chess_update({
      game_id = "hide01",
      seq = 1,
      to_act = { "bob" },
      view = {
        fen = AFTER_E4_FEN,
        history_san = { "e4" },
        legal_moves = {},
        claimable_draw = false,
      },
    })
  )
  assert_eq(true, shell.board(buf).dirty)
  local stale = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  assert(not table.concat(stale, "\n"):find("1. e4", 1, true))

  vim.api.nvim_set_current_buf(buf)
  helpers.wait_for(function()
    return shell.board(buf).dirty == false
  end, "the shown board to refit")
  local fresh = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  assert(table.concat(fresh, "\n"):find("1. e4", 1, true), vim.inspect(fresh))
end

---A 2 x 4 game on board(), so a headless 80 x 24 editor has room to scale it.
local SQUARE_GAME = {
  layout = function(_, ctx)
    return board_widget({
      grid = {
        rows = 2,
        cols = 4,
        shade = "checker",
        cell = function()
          return { "x" }
        end,
      },
    }, ctx)
  end,
}
local HEADER_AND_CHROME_LINES = 3 + 3

---Cell height of the shown square game, from the buffer.
---@param buf integer
---@return number
local function square_cell_height(buf)
  return (vim.api.nvim_buf_line_count(buf) - HEADER_AND_CHROME_LINES) / 2
end

---@param buf integer
---@return integer display width of the square game's grid
local function square_board_width(buf)
  local first = 4
  local rows =
    vim.api.nvim_buf_get_lines(buf, first - 1, first - 1 + 2 * square_cell_height(buf), false)
  return text_util.max_display_width(rows)
end

---Opens the square game in the current window, runs fn with its buffer, then cleans up.
---@param game_id string
---@param fn fun(buf: integer)
local function with_square_board(game_id, fn)
  package.preload["games.games.square"] = function()
    return SQUARE_GAME
  end
  local client = client_with("alice", { chess_update({ game_id = game_id, game = "square" }) })
  local buf = assert(shell.open(client, game_id))
  local ok, err = xpcall(fn, debug.traceback, buf)
  vim.cmd("silent! tabonly")
  vim.cmd("silent! only")
  vim.api.nvim_buf_delete(buf, { force = true })
  package.preload["games.games.square"] = nil
  assert(ok, err)
end

---@param buf integer
---@return fun(): integer renders whole-buffer renders since the call
local function count_renders(buf)
  local renders = 0
  vim.api.nvim_buf_attach(buf, false, {
    on_lines = function(_, _, _, first)
      renders = renders + (first == 0 and 1 or 0)
    end,
  })
  return function()
    return renders
  end
end

---Without an attached UI nothing redraws, so Neovim never checks for WinResized itself.
local function fire_win_resized()
  vim.api.nvim_exec_autocmds("WinResized", {})
end

function T.refit_renders_once_per_split_and_fits_the_narrowest_window()
  with_square_board("fit01", function(buf)
    local full_height = square_cell_height(buf)
    local renders = count_renders(buf)

    vim.cmd.vsplit()
    helpers.wait_for(function()
      return renders() > 0
    end, "the split to refit")
    vim.wait(50)
    assert_eq(1, renders())
    local narrowest =
      math.min(unpack(vim.tbl_map(vim.api.nvim_win_get_width, vim.fn.win_findbuf(buf))))
    assert(square_cell_height(buf) < full_height, "the split shrank the board")
    assert(
      square_board_width(buf) <= narrowest,
      ("%d > %d"):format(square_board_width(buf), narrowest)
    )
    assert_eq(false, vim.wo.wrap)
    assert_eq("no", vim.wo.signcolumn)

    vim.cmd("vertical resize 50")
    fire_win_resized()
    helpers.wait_for(function()
      return renders() > 1
    end, "the resize to refit")
    narrowest = math.min(unpack(vim.tbl_map(vim.api.nvim_win_get_width, vim.fn.win_findbuf(buf))))
    assert(
      square_board_width(buf) <= narrowest,
      ("%d > %d"):format(square_board_width(buf), narrowest)
    )
  end)
end

function T.a_small_window_in_another_tab_does_not_size_the_current_tab()
  with_square_board("tab01", function(buf)
    local full_height = square_cell_height(buf)
    vim.cmd.tabnew()
    vim.cmd.sbuffer(buf)
    vim.cmd.resize(8)
    fire_win_resized()
    helpers.wait_for(function()
      return square_cell_height(buf) < full_height
    end, "the small window to refit")
    local small_height = square_cell_height(buf)

    vim.cmd.tabprevious()
    helpers.wait_for(function()
      return square_cell_height(buf) > small_height
    end, "the first tab to refit to its own window")
  end)
end

---Splits off a 5-line window on buf, too short for its board, scrolled to topline 3.
---@param buf integer
local function scroll_a_too_tall_board(buf)
  vim.cmd.split()
  vim.cmd.resize(5)
  fire_win_resized()
  helpers.wait_for(function()
    return square_cell_height(buf) == 1
  end, "the board to shrink below the window")
  vim.wait(20)
  vim.fn.winrestview({ topline = 3, lnum = 4 })
  assert_eq(3, vim.fn.winsaveview().topline)
end

function T.a_refit_that_re_renders_scrolls_the_board_to_its_top()
  with_square_board("top01", function(buf)
    scroll_a_too_tall_board(buf)
    vim.cmd.resize(6)
    fire_win_resized()
    helpers.wait_for(function()
      return vim.fn.winsaveview().topline == 1
    end, "topline 1 after the refit")
    assert_eq(1, vim.fn.line("."))
  end)
end

function T.a_refit_with_nothing_changed_keeps_the_scroll()
  with_square_board("top02", function(buf)
    scroll_a_too_tall_board(buf)
    vim.api.nvim_exec_autocmds("WinEnter", {})
    vim.wait(50)
    assert_eq(3, vim.fn.winsaveview().topline)
    assert_eq(4, vim.fn.line("."))
  end)
end

---Counts vim.schedule calls while fn runs; the scheduled work still runs.
---@param fn fun()
---@return integer
local function count_schedules(fn)
  local schedule, count = vim.schedule, 0
  helpers.with_overrides({
    {
      vim,
      "schedule",
      function(callback)
        count = count + 1
        schedule(callback)
      end,
    },
  }, fn)
  return count
end

function T.a_burst_of_window_events_schedules_one_refit_and_none_for_hidden_boards()
  with_square_board("burst01", function(buf)
    -- vim.schedule runs in order, so once this runs the refit that open scheduled has run too,
    -- and only the tick can clear the dirty flag.
    local open_settled = false
    vim.schedule(function()
      open_settled = true
    end)
    assert(
      vim.wait(1000, function()
        return open_settled
      end),
      "the refit open scheduled"
    )
    local burst = count_schedules(function()
      for _, event in ipairs({ "WinEnter", "WinResized", "VimResized", "TabEnter" }) do
        vim.api.nvim_exec_autocmds(event, {})
      end
    end)
    assert_eq(1, burst)
    -- vim.schedule runs in order, so once this runs the refit that open scheduled has run too,
    -- and only the tick can clear the dirty flag.
    local open_settled = false
    vim.schedule(function()
      open_settled = true
    end)
    assert(
      vim.wait(1000, function()
        return open_settled
      end),
      "the refit open scheduled"
    )

    vim.cmd.enew()
    assert_eq(0, count_schedules(fire_win_resized))
    vim.api.nvim_set_current_buf(buf)
  end)
end

function T.header_always_has_its_status_line()
  local offered = chess_update({ draw_agreed = { "bob" } })
  local plain, plain_height = shell.render(chess_update({}), chess, ASCII, "alice")
  local _, offered_height = shell.render(offered, chess, ASCII, "alice")

  assert_eq("", plain.lines[3])
  assert_eq(3, plain_height)
  assert_eq(3, offered_height)
end

function T.changing_ambiwidth_refits_shown_boards()
  local client = client_with("alice", { chess_update({ game_id = "amb01" }) })
  local buf = assert(shell.open(client, "amb01"))
  local function text()
    return table.concat(vim.api.nvim_buf_get_lines(buf, 0, -1, false), "\n")
  end
  assert(text():find("·", 1, true), text())
  helpers.with_overrides({ { vim.o, "ambiwidth", "double" } }, function()
    helpers.wait_for(function()
      return not text():find("·", 1, true)
    end, "the board to redraw without the widened dot")
  end)
  helpers.wait_for(function()
    return text():find("·", 1, true) ~= nil
  end, "the dot back under ambiwidth=single")
  vim.api.nvim_buf_delete(buf, { force = true })
end

---@param marks games.Extmark[]
---@return games.Extmark[] in row, then column order
local function by_position(marks)
  table.sort(marks, function(a, b)
    return a.row ~= b.row and a.row < b.row or (a.row == b.row and a.col < b.col)
  end)
  return marks
end

---The first height lines of buf and their highlights.
---@param buf integer
---@param height integer
---@return string[]
---@return games.Extmark[]
local function painted_header(buf, height)
  local ns = vim.api.nvim_get_namespaces()["games.shell"]
  local marks = vim.api.nvim_buf_get_extmarks(
    buf,
    ns,
    { 0, 0 },
    { height - 1, -1 },
    { details = true }
  )
  return vim.api.nvim_buf_get_lines(buf, 0, height, false),
    by_position(vim.tbl_map(function(mark)
      local details = mark[4]
      return {
        row = mark[2],
        col = mark[3],
        end_col = details.end_col,
        hl = details.hl_group,
      }
    end, marks))
end

function T.a_clock_tick_rewrites_the_clocks_in_place_unless_the_column_or_board_changed()
  local elapsed_ms = 1500
  local wide_name = { { user = "王", seat = "white" }, { user = "bob", seat = "black" } }
  for i, case in ipairs({
    { remaining_ms = { alice = 300000, bob = 600000 }, in_place = true },
    { remaining_ms = { alice = 10500, bob = 300000 }, in_place = true },
    { remaining_ms = { alice = 600000, bob = 300000 }, in_place = false },
    { remaining_ms = { ["王"] = 10500, bob = 600000 }, players = wide_name, in_place = true },
    { remaining_ms = { alice = 300000, bob = 600000 }, dirty = true, in_place = false },
  }) do
    local players = case.players or chess_update({}).players
    local update = chess_update({
      game_id = "tick0" .. i,
      players = players,
      clock = {
        remaining_ms = case.remaining_ms,
        running = players[1].user,
        as_of_ms = timer.unix_ms(),
      },
    })
    local client = client_with("alice", { update })
    local buf = assert(shell.open(client, update.game_id))
    -- vim.schedule runs in order, so once this runs the refit that open scheduled has run too,
    -- and only the tick can clear the dirty flag.
    local open_settled = false
    vim.schedule(function()
      open_settled = true
    end)
    assert(
      vim.wait(1000, function()
        return open_settled
      end),
      "the refit open scheduled"
    )
    shell.board(buf).dirty = case.dirty == true
    local expected, height = shell.render(update, chess, ASCII, "alice", elapsed_ms)
    local expected_lines = vim.list_slice(expected.lines, 1, height)
    local expected_marks = by_position(vim.tbl_filter(function(mark)
      return mark.row < height
    end, expected.extmarks))
    local lines_changed, tick_done, repainted = {}, false, false
    local paint_update = paint.update
    vim.api.nvim_buf_attach(buf, false, {
      on_lines = function(_, _, _, first, old_last)
        if tick_done then
          return true
        end
        table.insert(lines_changed, old_last - first)
      end,
    })
    helpers.with_overrides({
      {
        timer,
        "unix_ms",
        function()
          return update.clock.as_of_ms + elapsed_ms
        end,
      },
      {
        paint,
        "update",
        function(...)
          repainted = true
          paint_update(...)
        end,
      },
    }, function()
      helpers.wait_for(function()
        return vim.deep_equal(expected_lines, (painted_header(buf, height)))
      end, "a clock tick")
    end)
    tick_done = true
    local _, marks = painted_header(buf, height)
    vim.api.nvim_buf_delete(buf, { force = true })

    assert_eq({ case, expected_marks }, { case, marks })
    local one_line_edits = vim.iter(lines_changed):all(function(count)
      return count == 1
    end)
    assert_eq(
      { case, case.in_place },
      { case, #lines_changed > 0 and one_line_edits and not repainted }
    )
  end
end

function T.an_update_after_a_clock_tick_repaints_the_clock_it_rewrote()
  local as_of_ms = timer.unix_ms()
  local function clocked(seq, at_ms)
    return chess_update({
      game_id = "tick10",
      seq = seq,
      clock = {
        remaining_ms = { alice = 300000, bob = 600000 },
        running = "alice",
        as_of_ms = at_ms,
      },
    })
  end
  local client, conn = client_with("alice", { clocked(1, as_of_ms) })
  local buf = assert(shell.open(client, "tick10"))
  local _, height = shell.render(clocked(1, as_of_ms), chess, ASCII, "alice", 0)
  local now_ms = as_of_ms + 1500
  local paint_update = paint.update
  local header_on_update
  helpers.with_overrides({
    {
      timer,
      "unix_ms",
      function()
        return now_ms
      end,
    },
    {
      paint,
      "update",
      function(...)
        paint_update(...)
        header_on_update = vim.api.nvim_buf_get_lines(buf, 0, height, false)
      end,
    },
  }, function()
    helpers.wait_for(function()
      return table.concat(vim.api.nvim_buf_get_lines(buf, 0, height, false)):find("4:59", 1, true)
        ~= nil
    end, "a clock tick")
    header_on_update = nil
    conn:push("game.update", clocked(2, now_ms))
    helpers.wait_for(function()
      return header_on_update ~= nil
    end, "the update painted")
  end)
  vim.api.nvim_buf_delete(buf, { force = true })

  local expected = shell.render(clocked(2, now_ms), chess, ASCII, "alice", 0)
  assert_eq(vim.list_slice(expected.lines, 1, height), header_on_update)
end

function T.an_act_entry_sends_the_action_its_view_options_build_and_binds_nothing_without_one()
  ---@type games.GameModule
  local module = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    actions = {
      {
        key = "<CR>",
        label = "go",
        act = function(update, view)
          local at = view.cursor
          return at
              and update.view.can_go
              and { kind = "go", cell = ("%d,%d"):format(at.row, at.col) }
            or nil
        end,
      },
    },
  }
  local update = chess_update({ game_id = "act001", seq = 2, view = { can_go = true } })
  local aimed =
    vim.tbl_extend("force", ASCII, { cursor = { row = 3, col = 4, orientation = "down" } })
  local function go_binding(state, view, catalogued)
    local _, _, bindings = shell.render(state, module, view, "alice", 0, nil, catalogued)
    return vim.iter(bindings):find(function(b)
      return b.key == "<CR>"
    end)
  end

  assert(not go_binding(update, ASCII, { go = true }), "bound with nothing to send")
  assert(not go_binding(update, aimed, { move = true }), "bound without the catalogue listing it")
  assert(not go_binding(chess_update({ view = { can_go = false } }), aimed, { go = true }))
  local acted = {}
  go_binding(update, aimed, { go = true }).run({
    update = update,
    client = {
      game_act = function(_, request)
        acted[#acted + 1] = request
      end,
    },
  })
  assert_eq({ { game_id = "act001", seq = 2, action = { kind = "go", cell = "3,4" } } }, acted)
end

function T.a_view_key_reads_the_boards_latest_update()
  local seen
  package.loaded["games.games.reach_test"] = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    actions = {
      {
        key = "l",
        label = "look",
        local_fn = function(view, update)
          seen = update.seq
          return view
        end,
      },
    },
  }
  local client =
    client_with("alice", { chess_update({ game_id = "reach01", game = "reach_test" }) })
  local buf = assert(shell.open(client, "reach01"))
  mapping(buf, "l").callback()
  package.loaded["games.games.reach_test"] = nil
  vim.api.nvim_buf_delete(buf, { force = true })
  assert_eq(chess_update({}).seq, seen)
end

function T.an_alias_is_mapped_like_its_key_but_left_out_of_the_footer()
  local looks = 0
  package.loaded["games.games.alias_test"] = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    actions = {
      {
        key = "x",
        aliases = { "<Left>" },
        label = "look",
        local_fn = function(view)
          looks = looks + 1
          return view
        end,
      },
    },
  }
  local client =
    client_with("alice", { chess_update({ game_id = "alias01", game = "alias_test" }) })
  local buf = assert(shell.open(client, "alias01"))
  mapping(buf, "<Left>").callback()
  mapping(buf, "x").callback()
  local footer = vim.api.nvim_buf_get_lines(buf, -2, -1, false)[1]
  package.loaded["games.games.alias_test"] = nil
  vim.api.nvim_buf_delete(buf, { force = true })
  assert_eq(2, looks)
  assert(footer:find("x look", 1, true) and not footer:find("Left", 1, true), footer)
end

function T.an_alias_may_not_take_a_shell_key()
  package.loaded["games.games.alias_clash"] = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    actions = {
      {
        key = "x",
        aliases = { "q" },
        label = "look",
        local_fn = function(view)
          return view
        end,
      },
    },
  }
  local client =
    client_with("alice", { chess_update({ game_id = "alias02", game = "alias_clash" }) })
  local ok, err = pcall(shell.open, client, "alias02")
  package.loaded["games.games.alias_clash"] = nil
  assert(not ok and tostring(err):find('"q" is reserved', 1, true), tostring(err))
end

function T.available_gates_view_keys_and_the_typed_input_as_it_does_actions()
  ---@type games.GameModule
  local module = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    input = {
      kind = "go",
      prompt = "Go to: ",
      available = function(update)
        return update.view.open
      end,
    },
    actions = {
      {
        key = "r",
        label = "rotate",
        available = function(update)
          return update.view.open
        end,
        local_fn = function(view)
          return view
        end,
      },
    },
  }
  local function footer(open)
    local rendered = shell.render(
      chess_update({ view = { open = open } }),
      module,
      ASCII,
      "alice",
      0,
      nil,
      { go = { kind = "go", description = "go", input = { field = "cell" } } }
    )
    return rendered.lines[#rendered.lines]
  end
  local controls = "R resign  D offer draw  A abort  "
  assert_eq(controls .. "m go to  r rotate  ]g next game  q close", footer(true))
  assert_eq(controls .. "]g next game  q close", footer(false))
end

function T.typed_input_goes_in_the_field_the_catalogue_names_and_only_for_the_player_to_act()
  ---@type games.GameModule
  local module = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    input = { kind = "go", prompt = "Go to: " },
  }
  local update = chess_update({ game_id = "go01", seq = 2 })
  ---@param field string
  ---@param me games.UserID
  ---@return games.KeyBinding|nil
  local function input_binding(field, me)
    local catalogued = { go = { kind = "go", description = "go", input = { field = field } } }
    local _, _, bindings = shell.render(update, module, ASCII, me, 0, nil, catalogued)
    return vim.iter(bindings):find(function(binding)
      return binding.key == "m"
    end)
  end
  local acted = {}
  local board = {
    update = update,
    me = "alice",
    client = {
      game_act = function(_, request)
        acted[#acted + 1] = request.action
      end,
    },
  }
  helpers.with_input_answers({ "c5", "d6" }, function()
    assert(input_binding("cell", "alice")).run(board)
    assert(input_binding("square", "alice")).run(board)
  end)

  assert_eq({ { kind = "go", cell = "c5" }, { kind = "go", square = "d6" } }, acted)
  assert(not input_binding("cell", "bob"), "bound for a player who is not to act")
end

function T.a_flippable_game_gets_the_flip_key_and_draws_flipped_for_its_bottom_seat()
  ---@type games.GameModule
  local module = {
    layout = function(_, ctx)
      return require("games.ui.compose").text(tostring(ctx.flip))
    end,
    flippable = { bottom_seat = "black" },
  }
  ---@param me games.UserID
  ---@param flipped boolean
  ---@return string[]
  local function render(me, flipped)
    local view = vim.tbl_extend("force", ASCII, { flipped = flipped })
    return shell.render(chess_update({}), module, view, me, 0).lines
  end
  local board_line = 5

  assert_eq("false", render("alice", false)[board_line])
  assert_eq("true", render("alice", true)[board_line])
  assert_eq("true", render("bob", false)[board_line])
  assert_eq("false", render("bob", true)[board_line])
  local footer = render("alice", false)[board_line + 2]
  assert(footer:find("F flip  ]g next game", 1, true), footer)

  package.loaded["games.games.flip_test"] = module
  local client = client_with("alice", { chess_update({ game_id = "flip01", game = "flip_test" }) })
  local buf = assert(shell.open(client, "flip01"))
  local board = assert(shell.board(buf))
  board.bindings.F.run(board)
  local flipped = vim.api.nvim_buf_get_lines(buf, board_line - 1, board_line, false)[1]
  package.loaded["games.games.flip_test"] = nil
  vim.api.nvim_buf_delete(buf, { force = true })
  assert_eq("true", flipped)
end

function T.a_flippable_game_may_not_bind_the_flip_key_itself()
  package.loaded["games.games.flip_clash"] = {
    layout = function()
      return require("games.ui.compose").text("board")
    end,
    flippable = { bottom_seat = "black" },
    actions = { { key = "F", label = "fold", action = { kind = "fold" } } },
  }
  local client = client_with("alice", { chess_update({ game_id = "flip02", game = "flip_clash" }) })
  local ok, err = pcall(shell.open, client, "flip02")
  package.loaded["games.games.flip_clash"] = nil
  assert(not ok and tostring(err):find('"F" is reserved', 1, true), tostring(err))
end

function T.a_game_with_asset_keys_needs_its_art()
  local ok, err = pcall(shell.render, chess_update({}), chess, { glyphs = "ascii" }, "alice", 0)

  assert(not ok and tostring(err):find("chess draws from an asset pack", 1, true), tostring(err))
end

return T
