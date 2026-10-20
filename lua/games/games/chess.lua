local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
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

local WHITE_PIECE_HL = "GamesPieceWhite"
local BLACK_PIECE_HL = "GamesPieceBlack"
local LAST_MOVE_HL = "GamesLastMove"
local LAST_MOVE_DARK_HL = "GamesLastMoveDark"
local CHECK_HL = "GamesCheck"
---@type games.MoveAnnotation
local CHECK_ANNOTATION = { pattern = "[+#]$", hl = "GamesCheckSign" }

---Display columns the move list reserves: "999. exd8=Q+ exd1=Q#".
local MOVELIST_WIDTH = 20

---@class (private) games.ChessLegendEntry
---@field piece? games.ChessPiece whose glyph the entry shows, when it is not the SAN letter itself
---@field san string
---@field name string

---@type games.ChessLegendEntry[]
local SAN_LEGEND = {
  { piece = "K", san = "K", name = "king" },
  { piece = "Q", san = "Q", name = "queen" },
  { piece = "R", san = "R", name = "rook" },
  { piece = "B", san = "B", name = "bishop" },
  { piece = "N", san = "N", name = "knight" },
  { piece = "P", san = " ", name = "pawn: e4" },
  { san = " ", name = "castle: O-O" },
}

---@param set games.ArtSet
---@return games.KvPair[]
local function san_legend(set)
  return vim.tbl_map(function(entry)
    local glyph = entry.piece and art.glyph(set, entry.piece)
    local key = (glyph and glyph ~= entry.piece) and ("%s %s"):format(glyph, entry.san) or entry.san
    return { key, entry.name }
  end, SAN_LEGEND)
end

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
---@return games.GridCell
local function cell_for(piece)
  if not piece then
    return { EMPTY_KEY }
  end
  return { piece, piece:match("%u") and WHITE_PIECE_HL or BLACK_PIECE_HL }
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
  local set = assert(ctx.art, "chess draws from an asset pack")
  return board({
    grid = {
      rows = 8,
      cols = 8,
      shade = "checker",
      flip = black_at_bottom(update, ctx),
      labels = { cols = FILES, rows = RANKS },
      cell = function(r, c)
        return cell_for(pieces[r][c])
      end,
      marks = marks_for(view, pieces),
    },
    art = set,
    panels = {
      {
        node = function()
          return kv({ pairs = san_legend(set), key_hl = "GamesKey", value_hl = "GamesLabel" })
        end,
      },
      {
        width = MOVELIST_WIDTH,
        node = function(max_height)
          return movelist({
            items = view.history_san,
            columns = 2,
            numbered = true,
            annotation = CHECK_ANNOTATION,
            max_lines = max_height,
          })
        end,
      },
    },
  }, ctx)
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
