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

local function stub_client(updates)
  local client = { cache = {}, listeners = {}, sent = {} }
  for _, update in ipairs(updates) do
    client.cache[update.game_id] = update
  end
  function client:game(game_id)
    return self.cache[game_id]
  end
  function client:games()
    return vim.tbl_values(self.cache)
  end
  function client:me()
    return "alice"
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
  local confirm = vim.fn.confirm
  local answer = 2
  vim.fn.confirm = function()
    return answer
  end

  mapping(buf, "R").callback()
  assert_eq({}, client.sent)
  answer = 1
  mapping(buf, "R").callback()
  vim.fn.confirm = confirm

  assert_eq({
    { type = "game.control", body = { game_id = "resign01", seq = 3, control = "resign" } },
  }, client.sent)
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
