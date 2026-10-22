local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local movelist = require("games.ui.widgets.movelist")

local Group = highlights.Group

---@class games.ChessSquares
---@field from string
---@field to string

---What a chess board draws, which chess puzzles share.
---@class games.ChessBoardView
---@field fen string
---@field history_san string[]
---@field last_move? games.ChessSquares
---@field check_square? string the king of the side to move, while it is in check
---@field first_ply integer plies the start position has behind it, which the move list numbers on from

---@class games.ChessView: games.ChessBoardView
---@field legal_moves string[] UCI, the viewer's own: empty unless it is to move
---@field claimable_draw boolean whether the viewer may claim a draw now
---@field material games.ChessMaterial

---@alias games.ChessSeat "white"|"black"

---@class games.ChessUpdate: games.GameUpdate
---@field view games.ChessView

---@class games.ChessMovedEvent: games.Event
---@field kind "moved"
---@field by games.ChessSeat
---@field san string

---@alias games.ChessPiece "K"|"Q"|"R"|"B"|"N"|"P"|"k"|"q"|"r"|"b"|"n"|"p"

---One side's material lead.
---@class games.ChessSideMaterial
---@field pieces games.ChessPiece[] the opponent's pieces this side is up, most valuable first
---@field score integer points this side is ahead; 0 unless it is ahead

---@class games.ChessMaterial
---@field white games.ChessSideMaterial
---@field black games.ChessSideMaterial

local FILES = "abcdefgh"
local RANKS = "87654321"
local EMPTY_KEY = "empty"
local MOVE_KIND = "move"

local WHITE_PIECE_HL = Group.PIECE_WHITE
local BLACK_PIECE_HL = Group.PIECE_BLACK
---@type games.MoveAnnotation
local CHECK_ANNOTATION = { pattern = "[+#]$", hl = Group.CHECK_SIGN }

local MATERIAL_HL = Group.MUTED

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
  local hl = grid.is_dark(row, col) and { Group.LAST_MOVE, Group.LAST_MOVE_DARK } or Group.LAST_MOVE
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
    marks[#marks + 1] = grid.mark(row, col, Group.CHECK)
  end
  return marks
end

local M = {}

---@type string[] keys every chess asset pack draws
M.asset_keys = { "K", "Q", "R", "B", "N", "P", "k", "q", "r", "b", "n", "p", EMPTY_KEY }

---The pieces seat is up, then its lead as +N when it is ahead.
---@param update games.ChessUpdate
---@param seat games.ChessSeat
---@param ctx games.ArtLayoutCtx
---@return games.TextNode[]|nil
function M.player_extra(update, seat, ctx)
  local side = update.view.material[seat]
  if #side.pieces == 0 then
    return nil
  end
  local glyphs = art.glyph_line(ctx.art, side.pieces, "", ctx.ambiwidth)
  local score = side.score > 0 and (" +%d"):format(side.score) or ""
  return { compose.text(glyphs .. score, MATERIAL_HL) }
end

---@class games.ChessBoardOpts
---@field panels? games.BoardPanel[] drawn after the move list

---The board of view, with the SAN legend and the move list beside it, then opts.panels.
---@param view games.ChessBoardView
---@param ctx games.ArtLayoutCtx
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
          return kv.panel(san_legend(ctx.art, ctx.ambiwidth))
        end,
      },
      movelist.panel({
        items = view.history_san,
        columns = PLIES_PER_MOVE,
        min_width = MOVELIST_WIDTH,
        offset = view.first_ply,
        annotation = CHECK_ANNOTATION,
      }),
    }, opts.panels or {}),
  }, ctx)
end

---@param update games.ChessUpdate
---@param ctx games.ArtLayoutCtx
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
M.flippable = {
  bottom_seat = function()
    return "black"
  end,
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  moved = function(event, talk)
    ---@cast event games.ChessMovedEvent
    return ("%s played %s"):format(talk.name(event.by), event.san)
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local white_king = { "K", WHITE_PIECE_HL }
  return {
    {
      sample = grid.samples(ctx, { white_king, { "k", BLACK_PIECE_HL } }),
      meaning = "white and black pieces, named in the panel",
    },
    grid.legend_entry(
      ctx,
      { EMPTY_KEY },
      "the last move: the squares it left and reached",
      Group.LAST_MOVE
    ),
    grid.legend_entry(ctx, white_king, "a king in check", Group.CHECK),
    {
      sample = { compose.text("+ #", CHECK_ANNOTATION.hl) },
      meaning = "check and checkmate in the move list",
    },
  }
end

return M
