local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local kv = require("games.ui.widgets.kv")
local movelist = require("games.ui.widgets.movelist")

---@class games.ChessSquares
---@field from string
---@field to string

---What a chess board draws, which chess puzzles share.
---@class games.ChessBoardView
---@field fen string
---@field history_san string[]
---@field last_move? games.ChessSquares
---@field check_square? string the king of the side to move, while it is in check

---@class games.ChessView: games.ChessBoardView
---@field legal_moves string[] UCI, the viewer's own: empty unless it is to move
---@field claimable_draw boolean whether the viewer may claim a draw now

---@alias games.ChessSeat "white"|"black"

---@class games.ChessUpdate: games.GameUpdate
---@field view games.ChessView

---@class games.ChessMovedEvent: games.Event
---@field kind "moved"
---@field by games.ChessSeat
---@field san string

---@alias games.ChessPiece "K"|"Q"|"R"|"B"|"N"|"P"|"k"|"q"|"r"|"b"|"n"|"p"

---One side's material lead.
---@class games.ChessMaterialSide
---@field pieces games.ChessPiece[] the opponent's pieces this side is up, most valuable first
---@field score integer points this side is ahead; 0 unless it is ahead

---@class games.ChessMaterial
---@field white games.ChessMaterialSide
---@field black games.ChessMaterialSide

local FILES = "abcdefgh"
local RANKS = "87654321"
local EMPTY_KEY = "empty"
local MOVE_KIND = "move"

local WHITE_PIECE_HL = "GamesPieceWhite"
local BLACK_PIECE_HL = "GamesPieceBlack"
local LAST_MOVE_HL = "GamesLastMove"
local LAST_MOVE_DARK_HL = "GamesLastMoveDark"
local CHECK_HL = "GamesCheck"
---@type games.MoveAnnotation
local CHECK_ANNOTATION = { pattern = "[+#]$", hl = "GamesCheckSign" }

local MATERIAL_HL = "GamesMuted"

---@class (private) games.ChessPieceKind
---@field white games.ChessPiece
---@field black games.ChessPiece
---@field points integer

---Counted kinds, most valuable first; the king is not counted.
---@type games.ChessPieceKind[]
local COUNTED_KINDS = {
  { white = "Q", black = "q", points = 9 },
  { white = "R", black = "r", points = 5 },
  { white = "B", black = "b", points = 3 },
  { white = "N", black = "n", points = 3 },
  { white = "P", black = "p", points = 1 },
}

---Display columns the move list reserves: "999. exd8=Q+ exd1=Q#".
local MOVELIST_WIDTH = 20
local PLIES_PER_MOVE = 2

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
---@param ambiwidth games.Ambiwidth
---@return games.KvPair[]
local function san_legend(set, ambiwidth)
  return vim.tbl_map(function(entry)
    local glyph = entry.piece and art.glyph(set, entry.piece, ambiwidth)
    local key = (glyph and glyph ~= entry.piece) and ("%s %s"):format(glyph, entry.san) or entry.san
    return { key, entry.name }
  end, SAN_LEGEND)
end

---@alias games.ChessPieces (games.ChessPiece|false)[][] rows from rank 8 down, false for an empty square

---@param fen string
---@return games.ChessPieces
local function parse_placement(fen)
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

---@param fen string
---@return table<games.ChessPiece, integer> pieces on the board, by piece
local function piece_counts(fen)
  return vim.iter(parse_placement(fen)):flatten():fold({}, function(counts, piece)
    if piece then
      counts[piece] = (counts[piece] or 0) + 1
    end
    return counts
  end)
end

---@param piece games.ChessPiece
---@param n integer
---@return games.ChessPiece[]
local function repeated(piece, n)
  local pieces = {}
  for i = 1, n do
    pieces[i] = piece
  end
  return pieces
end

---@param square string e.g. "e4"
---@return integer row
---@return integer col
local function square_cell(square)
  local file = FILES:find(square:sub(1, 1), 1, true)
  local rank = RANKS:find(square:sub(2, 2), 1, true)
  return assert(rank), assert(file)
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
---@return games.GridMark
local function last_move_mark(square)
  local row, col = square_cell(square)
  local hl = grid.is_dark(row, col) and { LAST_MOVE_HL, LAST_MOVE_DARK_HL } or LAST_MOVE_HL
  return grid.mark(row, col, hl)
end

---@param view games.ChessBoardView
---@return games.GridMark[]
local function marks_for(view)
  local marks = {}
  if view.last_move then
    marks[1] = last_move_mark(view.last_move.from)
    marks[2] = last_move_mark(view.last_move.to)
  end
  if view.check_square then
    local row, col = square_cell(view.check_square)
    marks[#marks + 1] = grid.mark(row, col, CHECK_HL)
  end
  return marks
end

local M = {}

---@type string[] keys every chess asset pack draws
M.asset_keys = { "K", "Q", "R", "B", "N", "P", "k", "q", "r", "b", "n", "p", EMPTY_KEY }

---The material imbalance on the board of fen: per kind, the surplus of the side with more of
---it, shown as the opponent's pieces it is up; equal counts cancel.
---@param fen string
---@return games.ChessMaterial
function M.material(fen)
  local counts = piece_counts(fen)
  local white_pieces, black_pieces, balance = {}, {}, 0
  for _, kind in ipairs(COUNTED_KINDS) do
    local surplus = (counts[kind.white] or 0) - (counts[kind.black] or 0)
    balance = balance + surplus * kind.points
    vim.list_extend(white_pieces, repeated(kind.black, math.max(surplus, 0)))
    vim.list_extend(black_pieces, repeated(kind.white, math.max(-surplus, 0)))
  end
  return {
    white = { pieces = white_pieces, score = math.max(balance, 0) },
    black = { pieces = black_pieces, score = math.max(-balance, 0) },
  }
end

---The pieces seat is up, then its lead as +N when it is ahead.
---@param update games.ChessUpdate
---@param seat games.ChessSeat
---@param ctx games.LayoutCtx
---@return games.TextNode[]|nil
function M.player_extra(update, seat, ctx)
  local side = M.material(update.view.fen)[seat]
  if #side.pieces == 0 then
    return nil
  end
  local glyphs = art.glyph_line(ctx.art --[[@as games.ArtSet]], side.pieces, "", ctx.ambiwidth)
  local score = side.score > 0 and (" +%d"):format(side.score) or ""
  return { compose.text(glyphs .. score, MATERIAL_HL) }
end

---How many plies a game starting from fen has behind it, read from its side to move and
---fullmove number, so its move list numbers on from there; 0 for "", the standard start.
---@param fen string
---@return integer
function M.first_ply(fen)
  local side, fullmove = fen:match("^%S+%s+([wb])%s+%S+%s+%S+%s+%d+%s+(%d+)")
  if not side then
    return 0
  end
  return (tonumber(fullmove) - 1) * PLIES_PER_MOVE + (side == "b" and 1 or 0)
end

---@class games.ChessBoardOpts
---@field first_ply? integer plies played before history_san's first, as M.first_ply reads them; 0 by default
---@field panels? games.BoardPanel[] drawn after the move list

---The board of view, with the SAN legend and the move list beside it, then opts.panels.
---@param view games.ChessBoardView
---@param ctx games.LayoutCtx
---@param flip boolean whether black is at the bottom
---@param opts? games.ChessBoardOpts
---@return games.Node
function M.board(view, ctx, flip, opts)
  opts = opts or {}
  local pieces = parse_placement(view.fen)
  return board({
    grid = {
      rows = 8,
      cols = 8,
      shade = "checker",
      flip = flip,
      labels = { cols = FILES, rows = RANKS },
      cell = function(r, c)
        return cell_for(pieces[r][c])
      end,
      marks = marks_for(view),
    },
    panels = vim.list_extend({
      {
        node = function()
          return kv.panel(san_legend(ctx.art --[[@as games.ArtSet]], ctx.ambiwidth))
        end,
      },
      movelist.panel({
        items = view.history_san,
        columns = PLIES_PER_MOVE,
        min_width = MOVELIST_WIDTH,
        offset = opts.first_ply,
        annotation = CHECK_ANNOTATION,
      }),
    }, opts.panels or {}),
  }, ctx)
end

---@param update games.ChessUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return M.board(update.view, ctx, ctx.flip)
end

---@type games.GameInput
M.input = { kind = MOVE_KIND, prompt = "Move: " }

---@type games.GameAction[]
M.actions = {
  {
    key = "C",
    label = "claim draw",
    action = { kind = "claim_draw" },
    available = function(update)
      return update.view.claimable_draw
    end,
  },
}

---@type games.Flippable
M.flippable = { bottom_seat = "black" }

---@type table<games.EventKind, games.Describer>
M.describers = {
  moved = function(event, talk)
    ---@cast event games.ChessMovedEvent
    return ("%s played %s"):format(talk.name(event.by), event.san)
  end,
}

return M
