local assets = require("games.ui.assets")
local checkers = require("games.games.checkers")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local MOVABLE = "GamesMovable"

---@param glyphs games.Glyphs
---@param flipped? boolean
---@return games.ViewOptions
local function view_options(glyphs, flipped)
  local spec = { keys = checkers.asset_keys, glyph_width = 1 }
  local pack = assets.load("checkers", assets.DEFAULT_PACK, spec)
  return { glyphs = glyphs, flipped = flipped == true, art = assets.art_set(pack, glyphs) }
end

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
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "checkers",
    seq = 0,
    options = {},
    players = { { user = "alice", seat = "black" }, { user = "bob", seat = "white" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw", "abort" },
    events = {},
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
  for _, mark in ipairs(rendered.extmarks) do
    if mark.row == header_height + row and mark.col == (col - 1) * 2 then
      return mark.hl
    end
  end
end

---@param rendered games.Rendered
---@return integer marks that tint a movable piece
local function movable_marks(rendered)
  return #vim.tbl_filter(function(mark)
    return type(mark.hl) == "table" and vim.tbl_contains(mark.hl, MOVABLE)
  end, rendered.extmarks)
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
    local rendered, header_height =
      shell.render(checkers_update({}), checkers, view_options("ascii"), case.me)
    assert_eq(case.lines, board_lines(rendered, header_height))
    assert_eq("- move  x jump", vim.trim(rendered.lines[header_height + 10]))
  end
end

function T.flip_mirrors_the_drawing_and_the_numbering_together()
  local rendered, header_height =
    shell.render(checkers_update({}), checkers, view_options("ascii", true), "bob")
  assert_eq("  w   w   w   w     32  31  30  29", board_lines(rendered, header_height)[1])
end

function T.unicode_draws_hollow_white_and_solid_black_with_kings()
  local board = start_board()
  board[1], board[32] = "B", "W"
  local rendered, header_height = shell.render(
    checkers_update({ view = { board = board, legal = {}, last = {}, history = {} } }),
    checkers,
    view_options("unicode"),
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

  local rendered, header_height = shell.render(update, checkers, view_options("ascii"), "bob")

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
    view_options("ascii"),
    "alice"
  )
  assert_eq(0, movable_marks(other))
end

function T.describe_event_names_the_player_and_a_crowning()
  local update = checkers_update({})
  assert_eq(
    "alice played 22x31, crowned",
    checkers.describe_event(
      { kind = "moved", by = "black", move = "22x31", crowned = true },
      update
    )
  )
  assert_eq(
    "bob played 22-18",
    checkers.describe_event(
      { kind = "moved", by = "white", move = "22-18", crowned = false },
      update
    )
  )
  assert_eq(nil, checkers.describe_event({ kind = "other" }, update))
end

function T.typed_move_is_sent_raw_for_the_server_to_parse()
  assert_eq({ kind = "move", text = "15x24x31" }, checkers.input.parse("15x24x31"))
end

return T
