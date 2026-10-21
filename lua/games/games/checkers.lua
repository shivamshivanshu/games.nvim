local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local movelist = require("games.ui.widgets.movelist")
local players = require("games.core.players")

---@alias games.CheckersPiece ""|"b"|"B"|"w"|"W" empty, black man, black king, white man, white king

---A playable square in standard numbering, 1 to 32: square 1 is b8 and 32 is g1.
---@alias games.CheckersSquare integer

---@class games.CheckersView
---@field board games.CheckersPiece[] 32 squares; index i is square i
---@field legal games.CheckersSquare[][] the viewer's own legal moves as paths, origin first: empty unless it is to move
---@field last games.CheckersSquare[] the path of the last move, empty before the first
---@field history string[] moves in notation, e.g. 11-15 or 15x24x31

---@alias games.CheckersSeat "black"|"white"

---@class games.CheckersUpdate: games.GameUpdate
---@field view games.CheckersView

---@class games.CheckersMovedEvent: games.Event
---@field kind "moved"
---@field by games.CheckersSeat
---@field move string
---@field crowned boolean

local SIZE = 8
local SQUARES_PER_ROW = SIZE / 2
---@type games.CheckersSeat
local BLACK = "black"
local EMPTY_KEY = "empty"
local LIGHT_KEY = "light"

local LAST_MOVE_HL = { "GamesLastMove", "GamesLastMoveDark" }
local MOVABLE_HL = "GamesMovable"
local NOTATION_HL = "GamesLabel"
local NUMBER_HL = "GamesMuted"
local NOTATION = "- move  x jump"

---@type table<games.CheckersPiece, games.Hl>
local PIECE_HL =
  { b = "GamesPieceBlack", B = "GamesPieceBlack", w = "GamesPieceWhite", W = "GamesPieceWhite" }

---Display columns the move list reserves at least: "99. 15x24x31x22 22x15x8".
local MOVELIST_WIDTH = 24

---@param row integer
---@param col integer
---@return games.CheckersSquare|nil nil for a light square
local function square_at(row, col)
  if not grid.is_dark(row, col) then
    return nil
  end
  return (row - 1) * SQUARES_PER_ROW + math.floor((col - 1) / 2) + 1
end

---@param square games.CheckersSquare
---@return integer row
---@return integer col
local function cell_of(square)
  local row = math.floor((square - 1) / SQUARES_PER_ROW) + 1
  local offset = 2 * ((square - 1) % SQUARES_PER_ROW)
  return row, row % 2 == 1 and offset + 2 or offset + 1
end

---@param square games.CheckersSquare
---@param hl games.Hl
---@return games.GridMark
local function mark_on(square, hl)
  local row, col = cell_of(square)
  return { row = row, col = col, hl = hl }
end

---The last move's path, then the pieces the viewer may move.
---@param view games.CheckersView
---@return games.GridMark[]
local function marks_for(view)
  local marks = vim.tbl_map(function(square)
    return mark_on(square, LAST_MOVE_HL)
  end, view.last)
  for _, path in ipairs(view.legal) do
    marks[#marks + 1] = mark_on(path[1], MOVABLE_HL)
  end
  return marks
end

---The square numbers laid out as the board is drawn, one line per row.
---@param flip boolean
---@return games.Node
local function numbering(flip)
  local order = grid.display_order(SIZE, flip)
  return compose.col(vim.tbl_map(function(row)
    local cells = vim.tbl_map(function(col)
      local square = square_at(row, col)
      return square and ("%2d"):format(square) or "  "
    end, order)
    return compose.text(table.concat(cells), NUMBER_HL)
  end, order))
end

---@param history string[]
---@param max_lines? integer
---@return games.Node
local function moves_node(history, max_lines)
  return movelist({ items = history, columns = 2, numbered = true, max_lines = max_lines })
end

---The move list's width over the whole game: MOVELIST_WIDTH unless long jump chains or move
---numbers past it need more, so an ordinary game never resizes the board.
---@param history string[]
---@return integer
local function movelist_width(history)
  return math.max(MOVELIST_WIDTH, compose.measure(moves_node(history)).w)
end

---@param update games.CheckersUpdate
---@param ctx games.LayoutCtx
---@return boolean
local function black_at_bottom(update, ctx)
  return (players.seat_of_user(update, ctx.me) == BLACK) ~= ctx.flipped
end

local M = {}

---@type string[] keys every checkers asset pack draws
M.asset_keys = { "b", "B", "w", "W", EMPTY_KEY, LIGHT_KEY }

---@param update games.CheckersUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local flip = black_at_bottom(update, ctx)
  return board({
    grid = {
      rows = SIZE,
      cols = SIZE,
      shade = "checker",
      flip = flip,
      cell = function(r, c)
        local square = square_at(r, c)
        if not square then
          return { LIGHT_KEY }
        end
        local piece = view.board[square]
        if piece == "" then
          return { EMPTY_KEY }
        end
        return { piece, PIECE_HL[piece] }
      end,
      marks = marks_for(view),
    },
    art = ctx.art,
    panels = {
      {
        node = function()
          return compose.col({ numbering(flip), compose.text(NOTATION, NOTATION_HL) })
        end,
      },
      {
        width = movelist_width(view.history),
        node = function(max_height)
          return moves_node(view.history, max_height)
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = "move",
  prompt = "Move: ",
  parse = function(text)
    return { kind = "move", text = text }
  end,
}

---@type games.GameAction[]
M.actions = {
  {
    key = "F",
    label = "flip",
    local_fn = function(view)
      return vim.tbl_extend("force", view, { flipped = not view.flipped })
    end,
  },
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  moved = function(event, talk)
    ---@cast event games.CheckersMovedEvent
    return ("%s played %s%s"):format(
      talk.name(event.by),
      event.move,
      event.crowned and ", crowned" or ""
    )
  end,
}

return M
