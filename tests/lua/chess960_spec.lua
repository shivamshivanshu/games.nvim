local chess960 = require("games.games.chess960")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local CASTLED_FEN = "rqnbnrk1/pppppbpp/8/5p2/5P2/8/PPPPPBPP/RQNBNRK1 w - - 4 4"
local FIRST_BOARD_ROW = 4
local BOARD_LINES = 9

---alice (white) and bob after both castled kingside from start position 413.
---@param overrides? table
---@return games.Chess960Update
local function castled(overrides)
  return helpers.chess_update(vim.tbl_extend("force", {
    game = "chess960",
    options = { time_control = "none", color = "white" },
    view = {
      fen = CASTLED_FEN,
      history_san = { "f4", "f5", "Bf2", "Bf7", "O-O", "O-O" },
      last_move = { from = "f8", to = "h8" },
      legal_moves = { "a2a3" },
      claimable_draw = false,
      start_position = 413,
    },
  }, overrides or {}))
end

---@param update games.Chess960Update
---@param me string
---@return string[]
local function board_lines(update, me)
  local rendered = helpers.render_game("chess960", update, { me = me })
  return vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + BOARD_LINES)
end

local T = {}

function T.board_shows_the_start_position_number_and_numbers_moves_from_the_start()
  assert_eq({
    "8 r q n b n r k .   K king          1. f4  f5    position 413",
    "7 p p p p p b p p   Q queen         2. Bf2 Bf7               ",
    "6 . . . . . . . .   R rook          3. O-O O-O               ",
    "5 . . . . . p . .   B bishop                                 ",
    "4 . . . . . P . .   N knight                                 ",
    "3 . . . . . . . .     pawn: e4                               ",
    "2 P P P P P B P P     castle: O-O                            ",
    "1 R Q N B N R K .                                            ",
    "  a b c d e f g h                                            ",
  }, board_lines(castled(), "alice"))
  assert_eq({
    "1 . K R N B N Q R   K king          1. f4  f5    position 413",
    "2 P P B P P P P P   Q queen         2. Bf2 Bf7               ",
    "3 . . . . . . . .   R rook          3. O-O O-O               ",
    "4 . . P . . . . .   B bishop                                 ",
    "5 . . p . . . . .   N knight                                 ",
    "6 . . . . . . . .     pawn: e4                               ",
    "7 p p b p p p p p     castle: O-O                            ",
    "8 . k r n b n q r                                            ",
    "  h g f e d c b a                                            ",
  }, board_lines(castled(), "bob"))
end

function T.claim_draw_is_offered_only_when_the_view_allows_it()
  for _, claimable in ipairs({ false, true }) do
    local update = castled()
    update.view.claimable_draw = claimable
    local rendered = helpers.render_game("chess960", update)
    assert_eq(claimable, helpers.footer(rendered):find(" C claim draw ", 1, true) ~= nil)
  end
end

function T.events_name_the_player_and_the_move()
  assert_eq(
    "bob played O-O",
    helpers.described(castled(), { kind = "moved", by = "black", san = "O-O" })
  )
end

return T
