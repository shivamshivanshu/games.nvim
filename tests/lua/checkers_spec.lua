local checkers = require("games.games.checkers")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local MOVABLE = "GamesMovable"

---@return games.CheckersPiece[]
local function start_board()
  local board = {}
  for square = 1, 32 do
    board[square] = square <= 12 and "b" or (square >= 21 and "w" or "")
  end
  return board
end

---@param overrides table
---@return games.CheckersUpdate
local function checkers_update(overrides)
  return helpers.game_update("checkers", {
    players = { { user = "alice", seat = "black" }, { user = "bob", seat = "white" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw", "abort" },
    view = { board = start_board(), legal = {}, last = {}, history = {} },
  }, overrides)
end

---@param rendered games.Rendered
---@param header_height integer
---@return string[] the eight board lines, trailing space trimmed
local function board_lines(rendered, header_height)
  return vim.tbl_map(function(line)
    return (line:gsub("%s+$", ""))
  end, vim.list_slice(rendered.lines, header_height + 2, header_height + 9))
end

---The highlight stack of an ascii cell drawn at display row and col, both 1-based.
---@param rendered games.Rendered
---@param header_height integer
---@param row integer
---@param col integer
---@return games.Hl|nil
local function hl_at(rendered, header_height, row, col)
  return helpers.hl_at(rendered, header_height + row, (col - 1) * 2)
end

---@param rendered games.Rendered
---@return integer marks that tint a movable piece
local function movable_marks(rendered)
  return #vim.tbl_filter(function(mark)
    return type(mark.hl) == "table" and vim.tbl_contains(mark.hl, MOVABLE)
  end, helpers.extmarks(rendered))
end

local T = {}

function T.white_sees_square_1_top_left_and_black_sees_the_board_turned_with_its_numbering()
  for _, case in ipairs({
    {
      me = "bob",
      lines = {
        "  b   b   b   b      1   2   3   4",
        "b   b   b   b      5   6   7   8",
        "  b   b   b   b      9  10  11  12",
        ".   .   .   .     13  14  15  16",
        "  .   .   .   .     17  18  19  20",
        "w   w   w   w     21  22  23  24",
        "  w   w   w   w     25  26  27  28",
        "w   w   w   w     29  30  31  32",
      },
    },
    {
      me = "alice",
      lines = {
        "  w   w   w   w     32  31  30  29",
        "w   w   w   w     28  27  26  25",
        "  w   w   w   w     24  23  22  21",
        ".   .   .   .     20  19  18  17",
        "  .   .   .   .     16  15  14  13",
        "b   b   b   b     12  11  10   9",
        "  b   b   b   b      8   7   6   5",
        "b   b   b   b      4   3   2   1",
      },
    },
  }) do
    local rendered, header_height = shell.render(
      checkers_update({}),
      checkers,
      helpers.view_options("checkers", "ascii"),
      case.me
    )
    assert_eq(case.lines, board_lines(rendered, header_height))
    assert_eq("- move  x jump", vim.trim(rendered.lines[header_height + 10]))
  end
end

function T.flip_mirrors_the_drawing_and_the_numbering_together()
  local rendered, header_height = shell.render(
    checkers_update({}),
    checkers,
    helpers.view_options("checkers", "ascii", { flipped = true }),
    "bob"
  )
  assert_eq("  w   w   w   w     32  31  30  29", board_lines(rendered, header_height)[1])
end

function T.unicode_draws_hollow_white_and_solid_black_with_kings()
  local board = start_board()
  board[1], board[32] = "B", "W"
  local rendered, header_height = shell.render(
    checkers_update({ view = { board = board } }),
    checkers,
    helpers.view_options("checkers", "unicode"),
    "bob"
  )
  local lines = board_lines(rendered, header_height)
  assert(vim.startswith(lines[1], "  ⛃   ⛂   ⛂   ⛂"), lines[1])
  assert(vim.startswith(lines[8], "⛀   ⛀   ⛀   ⛁"), lines[8])
end

function T.last_move_path_is_tinted_and_the_movers_pieces_are_marked_only_for_the_mover()
  local board = start_board()
  board[11], board[15] = "", "b"
  local view = {
    board = board,
    legal = { { 22, 18 }, { 22, 17 }, { 21, 17 } },
    last = { 11, 15 },
    history = { "11-15" },
  }
  local update = checkers_update({ seq = 1, to_act = { "bob" }, view = view })

  local rendered, header_height =
    shell.render(update, checkers, helpers.view_options("checkers", "ascii"), "bob")

  assert_eq(
    { "GamesSquareDark", "GamesLastMove", "GamesLastMoveDark", "GamesLastMoveDarkInk" },
    hl_at(rendered, header_height, 3, 6)
  )
  assert_eq(
    { "GamesSquareDark", "GamesLastMove", "GamesLastMoveDark", "GamesPieceBlack" },
    hl_at(rendered, header_height, 4, 5)
  )
  assert_eq({ "GamesSquareDark", MOVABLE, "GamesPieceWhite" }, hl_at(rendered, header_height, 6, 3))
  assert_eq({ "GamesSquareDark", MOVABLE, "GamesPieceWhite" }, hl_at(rendered, header_height, 6, 1))

  local waiting = vim.tbl_extend("force", view, { legal = {} })
  local other = shell.render(
    vim.tbl_extend("force", update, { view = waiting }),
    checkers,
    helpers.view_options("checkers", "ascii"),
    "alice"
  )
  assert_eq(0, movable_marks(other))
end

function T.long_jump_chains_and_move_numbers_past_99_stay_inside_the_window()
  local history = {}
  for i = 1, 200 do
    history[i] = i % 2 == 1 and "11-15" or "22-18"
  end
  vim.list_extend(history, { "10x17x26x19x10", "10x19x26x17x10" })
  local update = checkers_update({ view = { history = history } })
  local area = { w = 80, h = 30 }

  local rendered = shell.render(
    update,
    checkers,
    helpers.view_options("checkers", "unicode"),
    "bob",
    0,
    { text_area = area, seats = 2 }
  )

  local widest = vim.iter(rendered.lines):fold(0, function(widest, line)
    return math.max(widest, vim.fn.strdisplaywidth(line))
  end)
  assert(widest <= area.w, ("widest line %d > %d"):format(widest, area.w))
  assert(
    vim.iter(rendered.lines):any(function(line)
      return line:find("101. 10x17x26x19x10 10x19x26x17x10", 1, true) ~= nil
    end),
    "the long chains are not shown whole"
  )
end

function T.events_name_the_player_and_a_crowning()
  local update = checkers_update({})
  assert_eq(
    "alice played 22x31, crowned",
    helpers.described(update, { kind = "moved", by = "black", move = "22x31", crowned = true })
  )
  assert_eq(
    "bob played 22-18",
    helpers.described(update, { kind = "moved", by = "white", move = "22-18", crowned = false })
  )
end

return T
