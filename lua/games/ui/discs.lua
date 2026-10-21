local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local players = require("games.core.players")

---@alias games.DiscSeat "black"|"white"

---A cell on the wire, 1-based; named on screen as its column's letter then its row's number, e.g. h8.
---@class games.DiscCell
---@field row integer 1 at the top
---@field column integer 1 at the left

---@class games.DiscView
---@field cells (games.DiscSeat|"")[][] rows from the top; "" for an empty cell
---@field last? games.DiscCell where the latest disc went

---@class games.DiscBoardProps
---@field size integer cells on a side
---@field legal? games.DiscCell[] empty cells drawn as the viewer's moves; nil draws none
---@field winning_lines? games.DiscCell[][]

---@enum games.DiscKey asset keys besides the seats' own
local Key = { EMPTY = "empty", LEGAL = "legal" }

---@type table<games.DiscSeat, games.Hl>
local SEAT_HL = { black = "GamesPieceBlack", white = "GamesPieceWhite" }
local LAST_MOVE_HL = "GamesLastMove"
local WIN_HL = "GamesWin"
local LEGAL_HL = "GamesMovable"
local NOTE_HL = "GamesLabel"
local CELL_HL = "GamesKey"
local PLACE_KIND = "place"
local COLUMN_LETTERS = "abcdefghijklmnopqrstuvwxyz"
---Display columns the notes panel reserves: "line a15-o15".
local NOTES_WIDTH = 12

---Boards of black and white discs that players place by naming a cell, as gomoku and reversi do:
---lettered columns, numbered rows, and the cell typed at a prompt.
local M = {}

M.Key = Key

---The asset keys every pack for these boards draws: both seats' discs and the empty cell. A game
---that passes legal cells adds Key.LEGAL.
---@return string[]
function M.asset_keys()
  return { "black", "white", Key.EMPTY }
end

---@param cell games.DiscCell
---@return string
function M.cell_name(cell)
  return COLUMN_LETTERS:sub(cell.column, cell.column) .. cell.row
end

---@param update games.GameUpdate
---@param seat games.DiscSeat
---@return string
function M.who(update, seat)
  return players.user_of_seat(update, seat) or seat
end

---The prompt that sends the typed cell to the server, which parses it.
---@param example string a cell to show, e.g. "h8"
---@return games.GameInput
function M.input(example)
  return {
    kind = PLACE_KIND,
    prompt = ("Place, e.g. %s: "):format(example),
    parse = function(text)
      return { kind = PLACE_KIND, cell = text }
    end,
  }
end

---@param cells games.DiscCell[]
---@return table<integer, table<integer, true>> row -> column -> true
local function cell_set(cells)
  local set = {}
  for _, cell in ipairs(cells) do
    set[cell.row] = set[cell.row] or {}
    set[cell.row][cell.column] = true
  end
  return set
end

---Legal cells, then the last disc, then the winning lines over it; a later mark replaces an earlier one.
---@param view games.DiscView
---@param props games.DiscBoardProps
---@return games.GridMark[]
local function marks_for(view, props)
  local marks = {}
  local function mark(cell, hl)
    marks[#marks + 1] = { row = cell.row, col = cell.column, hl = hl }
  end
  for _, cell in ipairs(props.legal or {}) do
    mark(cell, LEGAL_HL)
  end
  if view.last then
    mark(view.last, LAST_MOVE_HL)
  end
  for _, line in ipairs(props.winning_lines or {}) do
    for _, cell in ipairs(line) do
      mark(cell, WIN_HL)
    end
  end
  return marks
end

---The last disc and each winning line in words, so neither rests on colour alone.
---@param view games.DiscView
---@param props games.DiscBoardProps
---@return games.Node
local function notes(view, props)
  local function note(label, cells)
    return compose.row({ compose.text(label .. " ", NOTE_HL), compose.text(cells, CELL_HL) })
  end
  local lines = {}
  if view.last then
    lines[1] = note("last", M.cell_name(view.last))
  end
  for _, line in ipairs(props.winning_lines or {}) do
    lines[#lines + 1] = note("line", M.cell_name(line[1]) .. "-" .. M.cell_name(line[#line]))
  end
  return compose.col(lines)
end

---@param size integer
---@return games.GridLabels
local function labels(size)
  return {
    cols = COLUMN_LETTERS:sub(1, size),
    rows = vim.tbl_map(tostring, vim.fn.range(1, size)),
  }
end

---The board, with the notes beside it.
---@param update games.GameUpdate
---@param ctx games.LayoutCtx
---@param props games.DiscBoardProps
---@return games.Node
function M.layout(update, ctx, props)
  ---@type games.DiscView
  local view = update.view
  local legal = cell_set(props.legal or {})
  return board({
    grid = {
      rows = props.size,
      cols = props.size,
      shade = "plain",
      labels = labels(props.size),
      cell = function(r, c)
        local seat = view.cells[r][c]
        if seat ~= "" then
          return { seat, SEAT_HL[seat] }
        end
        return { (legal[r] and legal[r][c]) and Key.LEGAL or Key.EMPTY }
      end,
      marks = marks_for(view, props),
    },
    art = ctx.art,
    panels = {
      {
        width = NOTES_WIDTH,
        node = function()
          return notes(view, props)
        end,
      },
    },
  }, ctx)
end

return M
