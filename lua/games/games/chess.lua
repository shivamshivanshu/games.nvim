local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local movelist = require("games.ui.widgets.movelist")
local players = require("games.core.players")

---@class games.ChessSquares
---@field from string
---@field to string

---@class games.ChessView
---@field fen string
---@field history_san string[]
---@field last_move? games.ChessSquares
---@field in_check boolean

---@alias games.ChessPiece "K"|"Q"|"R"|"B"|"N"|"P"|"k"|"q"|"r"|"b"|"n"|"p"

local FILES = "abcdefgh"
local RANKS = "87654321"
local BLACK = "black"
local KING_TO_MOVE = { w = "K", b = "k" }

---@type table<games.ChessPiece, string>
local UNICODE_PIECES = {
  K = "♔",
  Q = "♕",
  R = "♖",
  B = "♗",
  N = "♘",
  P = "♙",
  k = "♚",
  q = "♛",
  r = "♜",
  b = "♝",
  n = "♞",
  p = "♟",
}

---@type table<games.Glyphs, string>
local EMPTY_SQUARE = { unicode = "·", ascii = "." }

---@param fen string
---@return (games.ChessPiece|false)[][] rows from rank 8 down, false for an empty square
local function fen_to_pieces(fen)
  local placement = vim.split(fen, " ", { plain = true })[1]
  return vim.tbl_map(function(rank)
    local squares = {}
    for char in rank:gmatch(".") do
      local empty_run = tonumber(char)
      if empty_run then
        for _ = 1, empty_run do
          squares[#squares + 1] = false
        end
      else
        squares[#squares + 1] = char
      end
    end
    return squares
  end, vim.split(placement, "/", { plain = true }))
end

---@param row integer
---@param col integer
---@return string
local function mark_key(row, col)
  return row .. "," .. col
end

---@param square string e.g. "e4"
---@return string grid mark key
local function square_key(square)
  local file = FILES:find(square:sub(1, 1), 1, true)
  local rank = RANKS:find(square:sub(2, 2), 1, true)
  return mark_key(assert(rank), assert(file))
end

---@param pieces (games.ChessPiece|false)[][]
---@param piece games.ChessPiece
---@return string|nil grid mark key
local function find_piece(pieces, piece)
  for r, squares in ipairs(pieces) do
    for c, occupant in ipairs(squares) do
      if occupant == piece then
        return mark_key(r, c)
      end
    end
  end
end

---@param piece games.ChessPiece|false
---@param glyphs games.Glyphs
---@return string
local function glyph_for(piece, glyphs)
  if not piece then
    return EMPTY_SQUARE[glyphs]
  end
  return glyphs == "unicode" and UNICODE_PIECES[piece] or piece
end

---@param view games.ChessView
---@param pieces (games.ChessPiece|false)[][]
---@return table<string, string>
local function marks_for(view, pieces)
  local marks = {}
  if view.last_move then
    marks[square_key(view.last_move.from)] = "GamesLastMove"
    marks[square_key(view.last_move.to)] = "GamesLastMove"
  end
  if view.in_check then
    local side_to_move = vim.split(view.fen, " ", { plain = true })[2]
    local king = find_piece(pieces, KING_TO_MOVE[side_to_move])
    if king then
      marks[king] = "GamesCheck"
    end
  end
  return marks
end

---@param update games.GameUpdate
---@param ctx games.LayoutCtx
---@return boolean
local function black_at_bottom(update, ctx)
  return (players.seat_of_user(update, ctx.me) == BLACK) ~= ctx.flipped
end

local M = {}

---@param update games.GameUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view --[[@as games.ChessView]]
  local pieces = fen_to_pieces(view.fen)
  return compose.row({
    grid({
      rows = 8,
      cols = 8,
      shade = "checker",
      flip = black_at_bottom(update, ctx),
      labels = { cols = FILES, rows = RANKS },
      cell = function(r, c)
        return { glyph_for(pieces[r][c], ctx.glyphs) }
      end,
      marks = marks_for(view, pieces),
    }),
    compose.pad(3, movelist({ items = view.history_san, columns = 2, numbered = true })),
  })
end

M.input = {
  prompt = "Move: ",
  parse = function(text)
    return { kind = "move", move = text }
  end,
}

---@type games.GameAction[]
M.actions = {
  { key = "C", label = "claim draw", action = { kind = "claim_draw" } },
  {
    key = "F",
    label = "flip",
    local_fn = function(view)
      return vim.tbl_extend("force", view, { flipped = not view.flipped })
    end,
  },
}

---@param event games.Event
---@param update games.GameUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "moved" then
    return nil
  end
  return ("%s played %s"):format(players.user_of_seat(update, event.by) or event.by, event.san)
end

return M
