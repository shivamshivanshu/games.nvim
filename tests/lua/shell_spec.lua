local shell = require("games.ui.shell")
local chess = require("games.games.chess")

local START_FEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
local AFTER_E4_FEN = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1"
local FOOLS_MATE_FEN = "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3"
local ASCII = { glyphs = "ascii", flipped = false }

local function assert_eq(expected, actual)
  assert(vim.deep_equal(expected, actual), vim.inspect({ expected = expected, actual = actual }))
end

local function chess_update(overrides)
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "chess",
    seq = 0,
    options = {},
    players = { { user = "alice", seat = "white" }, { user = "bob", seat = "black" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw", "abort" },
    events = {},
    view = { fen = START_FEN, history_san = {}, in_check = false },
  }, overrides)
end

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
      local col = 2 + (file - 1) * 2
      for _, mark in ipairs(rendered.extmarks) do
        if mark.row == i - 1 and mark.col == col then
          return mark.hl
        end
      end
    end
  end
end

local function stub_client(updates, me)
  local client = { cache = {}, stamps = {}, listeners = {}, sent = {}, user = me or "alice" }
  for _, update in ipairs(updates) do
    client.cache[update.game_id] = update
    client.stamps[update.game_id] = vim.uv.now()
  end
  function client:game(game_id)
    return self.cache[game_id]
  end
  function client:games()
    return vim.tbl_values(self.cache)
  end
  function client:me()
    return self.user
  end
  function client:received_at(game_id)
    return self.stamps[game_id]
  end
  function client:on_update(fn)
    table.insert(self.listeners, fn)
  end
  function client:game_act(body)
    table.insert(self.sent, { type = "game.act", body = body })
  end
  function client:game_control(body)
    table.insert(self.sent, { type = "game.control", body = body })
  end
  function client:push(update)
    self.cache[update.game_id] = update
    self.stamps[update.game_id] = vim.uv.now()
    for _, fn in ipairs(self.listeners) do
      fn(update, false)
    end
  end
  return client
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

local function with_stub(tbl, key, value, fn)
  local original = tbl[key]
  tbl[key] = value
  local ok, err = pcall(fn)
  tbl[key] = original
  assert(ok, err)
end

local T = {}

function T.render_start_position()
  local rendered = shell.render(chess_update({}), chess, ASCII, "alice")

  local header = rendered.lines[1]
  assert(header:find("alice", 1, true) and header:find("← your move", 1, true), header)
  assert_eq("8 r n b q k b n r", without_trailing_space(board_lines(rendered.lines)[1]))
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
    result = { winners = { "bob" }, reason = "checkmate" },
    view = {
      fen = FOOLS_MATE_FEN,
      history_san = { "f3", "e5", "g4", "Qh4#" },
      last_move = { from = "d8", to = "h4" },
      in_check = true,
    },
  })
  local rendered = shell.render(update, chess, ASCII, "alice")

  assert_eq("GamesLastMove", hl_at_square(rendered, "d8"))
  assert_eq("GamesLastMove", hl_at_square(rendered, "h4"))
  assert_eq("GamesCheck", hl_at_square(rendered, "e1"))
  assert(table.concat(rendered.lines, "\n"):find("bob won", 1, true), vim.inspect(rendered.lines))
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
  local client = stub_client({ chess_update({ game_id = "keys01" }) })
  local buf = shell.open(client, "keys01")

  assert_eq("games://chess/bob/keys", vim.api.nvim_buf_get_name(buf))
  assert(mapping(buf, "R") and not mapping(buf, "X"))
  client:push(chess_update({
    game_id = "keys01",
    seq = 1,
    controls = { "resign", "offer_draw", "decline_draw" },
    draw_agreed = { "bob" },
  }))
  assert(mapping(buf, "X") and not mapping(buf, "A"))
end

function T.resign_confirms_then_sends_control()
  local client = stub_client({ chess_update({ game_id = "resign01", seq = 3 }) })
  local buf = shell.open(client, "resign01")
  local answer = 2
  with_stub(vim.fn, "confirm", function()
    return answer
  end, function()
    mapping(buf, "R").callback()
    assert_eq({}, client.sent)
    answer = 1
    mapping(buf, "R").callback()
  end)

  assert_eq({
    { type = "game.control", body = { game_id = "resign01", seq = 3, control = "resign" } },
  }, client.sent)
end

function T.flip_toggles_black_default_perspective()
  local client = stub_client({ chess_update({ game_id = "flip01" }) }, "bob")
  local buf = shell.open(client, "flip01")
  assert(vim.startswith(first_board_line(buf), "1 "), first_board_line(buf))

  mapping(buf, "F").callback()
  assert(vim.startswith(first_board_line(buf), "8 "), first_board_line(buf))
end

function T.open_charges_clock_from_cache_arrival()
  local update = chess_update({
    game_id = "clock01",
    clock = { remaining_ms = { alice = 300000, bob = 300000 }, running = "alice" },
  })
  local client = stub_client({ update })
  client.stamps.clock01 = vim.uv.now() - 61000
  local buf = shell.open(client, "clock01")
  local header = vim.api.nvim_buf_get_lines(buf, 0, 1, false)[1]
  vim.api.nvim_buf_delete(buf, { force = true })

  assert(header:find("3:59", 1, true), header)
end

function T.open_refuses_while_disconnected()
  local client = stub_client({ chess_update({ game_id = "offline01" }) })
  client.user = nil
  local messages = {}
  with_stub(vim, "notify", function(message)
    table.insert(messages, message)
  end, function()
    assert_eq(nil, shell.open(client, "offline01"))
  end)

  assert_eq({ "games: not connected" }, messages)
end

function T.reopen_after_bdelete_builds_fresh_board()
  local client = stub_client({ chess_update({ game_id = "bdel01" }) })
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
  local client = stub_client({ chess_update({ game_id = "hide01" }) })
  local buf = shell.open(client, "hide01")
  vim.cmd.enew()

  client:push(chess_update({
    game_id = "hide01",
    seq = 1,
    to_act = { "bob" },
    view = { fen = AFTER_E4_FEN, history_san = { "e4" }, in_check = false },
  }))
  assert_eq(true, vim.b[buf].games_dirty)
  local stale = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  assert(not table.concat(stale, "\n"):find("e4", 1, true))

  vim.api.nvim_set_current_buf(buf)
  assert_eq(false, vim.b[buf].games_dirty)
  local fresh = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  assert(table.concat(fresh, "\n"):find("1. e4", 1, true), vim.inspect(fresh))
end

return T
