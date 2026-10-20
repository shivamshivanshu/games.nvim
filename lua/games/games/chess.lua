local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local kv = require("games.ui.widgets.kv")
local movelist = require("games.ui.widgets.movelist")
local pgn = require("games.games.chess.pgn")
local players = require("games.core.players")

---@class games.ChessSquares
---@field from string
---@field to string

---@class games.ChessView
---@field fen string
---@field history_san string[]
---@field last_move? games.ChessSquares
---@field in_check boolean

---@alias games.ChessSeat "white"|"black"

---@class games.ChessUpdate: games.GameUpdate
---@field view games.ChessView

---@class games.ChessMovedEvent: games.Event
---@field kind "moved"
---@field by games.ChessSeat
---@field san string

---@alias games.ChessPiece "K"|"Q"|"R"|"B"|"N"|"P"|"k"|"q"|"r"|"b"|"n"|"p"

local FILES = "abcdefgh"
local RANKS = "87654321"
---@type games.ChessSeat
local BLACK = "black"
local KING_TO_MOVE = { w = "K", b = "k" }
local EMPTY_KEY = "empty"

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
local WHITE_PIECE_HL = "GamesPieceWhite"
local BLACK_PIECE_HL = "GamesPieceBlack"
local LAST_MOVE_HL = "GamesLastMove"
local LAST_MOVE_DARK_HL = "GamesLastMoveDark"
local CHECK_HL = "GamesCheck"
---@type games.MoveAnnotation
local CHECK_ANNOTATION = { pattern = "[+#]$", hl = "GamesCheckSign" }

local SAN_PIECE_NAMES = {
  { piece = "K", name = "king" },
  { piece = "Q", name = "queen" },
  { piece = "R", name = "rook" },
  { piece = "B", name = "bishop" },
  { piece = "N", name = "knight" },
}

---@param glyphs games.Glyphs
---@return games.KvPair[]
local function san_legend(glyphs)
  local function key(glyph, letter)
    return glyphs == "unicode" and ("%s %s"):format(glyph, letter) or letter
  end
  local legend = vim.tbl_map(function(entry)
    return { key(UNICODE_PIECES[entry.piece], entry.piece), entry.name }
  end, SAN_PIECE_NAMES)
  table.insert(legend, { key(UNICODE_PIECES.P, " "), "pawn: e4" })
  table.insert(legend, { key(" ", " "), "castle: O-O" })
  return legend
end

---@type table<games.Glyphs, games.KvPair[]>
local SAN_LEGEND = { unicode = san_legend("unicode"), ascii = san_legend("ascii") }

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

---@param square string e.g. "e4"
---@return integer row
---@return integer col
local function square_cell(square)
  local file = FILES:find(square:sub(1, 1), 1, true)
  local rank = RANKS:find(square:sub(2, 2), 1, true)
  return assert(rank), assert(file)
end

---@param pieces (games.ChessPiece|false)[][]
---@param piece games.ChessPiece
---@return string|nil grid mark key
local function find_piece(pieces, piece)
  for r, squares in ipairs(pieces) do
    for c, occupant in ipairs(squares) do
      if occupant == piece then
        return grid.mark_key(r, c)
      end
    end
  end
end

---@param piece games.ChessPiece|false
---@param ctx games.LayoutCtx
---@return games.GridCell
local function cell_for(piece, ctx)
  if not piece then
    return { EMPTY_SQUARE[ctx.glyphs] }
  end
  local glyph = ctx.glyphs == "unicode" and UNICODE_PIECES[piece] or piece
  return { glyph, piece:match("%u") and WHITE_PIECE_HL or BLACK_PIECE_HL }
end

---The last-move tint, with its darker shade layered on over a dark square.
---@param square string
---@return string key
---@return games.Hl
local function last_move_mark(square)
  local row, col = square_cell(square)
  local hl = grid.is_dark(row, col) and { LAST_MOVE_HL, LAST_MOVE_DARK_HL } or LAST_MOVE_HL
  return grid.mark_key(row, col), hl
end

---@param view games.ChessView
---@param pieces (games.ChessPiece|false)[][]
---@return table<string, games.Hl>
local function marks_for(view, pieces)
  local marks = {}
  if view.last_move then
    for _, square in ipairs({ view.last_move.from, view.last_move.to }) do
      local key, hl = last_move_mark(square)
      marks[key] = hl
    end
  end
  if view.in_check then
    local side_to_move = vim.split(view.fen, " ", { plain = true })[2]
    local king = find_piece(pieces, KING_TO_MOVE[side_to_move])
    if king then
      marks[king] = CHECK_HL
    end
  end
  return marks
end

---@param update games.ChessUpdate
---@param ctx games.LayoutCtx
---@return boolean
local function black_at_bottom(update, ctx)
  return (players.seat_of_user(update, ctx.me) == BLACK) ~= ctx.flipped
end

local M = {}

---@type string[] keys every chess asset pack draws
M.asset_keys = { "K", "Q", "R", "B", "N", "P", "k", "q", "r", "b", "n", "p", EMPTY_KEY }

---@param update games.ChessUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local pieces = fen_to_pieces(view.fen)
  return compose.row({
    grid({
      rows = 8,
      cols = 8,
      shade = "checker",
      flip = black_at_bottom(update, ctx),
      labels = { cols = FILES, rows = RANKS },
      cell = function(r, c)
        return cell_for(pieces[r][c], ctx)
      end,
      marks = marks_for(view, pieces),
    }),
    compose.pad(
      3,
      kv({
        pairs = SAN_LEGEND[ctx.glyphs],
        key_hl = "GamesKey",
        value_hl = "GamesLabel",
      })
    ),
    compose.pad(
      3,
      movelist({
        items = view.history_san,
        columns = 2,
        numbered = true,
        annotation = CHECK_ANNOTATION,
      })
    ),
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
---@param update games.ChessUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "moved" then
    return nil
  end
  ---@cast event games.ChessMovedEvent
  return ("%s played %s"):format(players.user_of_seat(update, event.by) or event.by, event.san)
end

---@param update games.ChessUpdate
---@return table<string, string>
function M.export(update)
  if not update.view then
    return {}
  end
  return { ["game.pgn"] = pgn.build(update) }
end

return M
