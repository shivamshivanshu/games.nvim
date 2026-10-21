local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local notation = require("games.ui.notation")
local text = require("games.ui.text")

local Group = highlights.Group

---@alias games.DiscSeat "black"|"white"

---@class games.DiscLast
---@field last? games.Cell where the latest disc went

---@class games.DiscView: games.DiscLast
---@field cells (games.DiscSeat|"")[][] rows from the top; "" for an empty cell

---@class games.DiscMarks what a board marks besides the last disc
---@field legal? games.Cell[] empty cells drawn as the viewer's moves; nil draws none
---@field winning_lines? games.Cell[][]

---@class games.DiscBoardProps: games.DiscMarks
---@field size integer cells on a side

local EMPTY_KEY = "empty"
local LEGAL_KEY = "legal"

---@type table<games.DiscSeat, games.Hl>
local SEAT_HL = { black = Group.PIECE_BLACK, white = Group.PIECE_WHITE }
local LEGAL_HL = Group.MOVABLE
local NOTE_HL = Group.LABEL
local CELL_HL = Group.KEY
local PLACE_KIND = "place"
---Display columns the notes panel reserves: "line a15-o15".
local NOTES_WIDTH = 12

---@enum games.DiscMarkKind marks a board may draw besides the discs and the last disc
local MarkKind = { LEGAL = "legal", WINNING_LINE = "winning_line" }

---Boards of black and white discs that players place by naming a cell, as gomoku and reversi do:
---lettered columns, numbered rows, and the cell typed at a prompt.
local M = {}

M.LEGAL_KEY = LEGAL_KEY
M.MarkKind = MarkKind

---The asset keys every pack for these boards draws: both seats' discs and the empty cell. A game
---that passes legal cells adds LEGAL_KEY.
---@return string[]
function M.asset_keys()
  return { "black", "white", EMPTY_KEY }
end

---A cell named as its column's letter then its row's number, e.g. h8.
---@param cell games.Cell
---@return string
function M.cell_name(cell)
  return text.ALPHABET:sub(cell.col, cell.col) .. cell.row
end

---The prompt that sends the typed cell to the server, which parses it.
---@param example string a cell to show, e.g. "h8"
---@return games.GameInput
function M.input(example)
  return { kind = PLACE_KIND, prompt = ("Place, e.g. %s: "):format(example) }
end

---Legal cells, then the last disc, then the winning lines over it; a later mark replaces an
---earlier one.
---@param view games.DiscLast
---@param marked games.DiscMarks
---@return games.GridMark[]
function M.marks(view, marked)
  local marks = grid.cell_marks(marked.legal or {}, LEGAL_HL)
  if view.last then
    marks[#marks + 1] = grid.mark(view.last.row, view.last.col, Group.LAST_MOVE)
  end
  for _, line in ipairs(marked.winning_lines or {}) do
    vim.list_extend(marks, grid.cell_marks(line, Group.WIN))
  end
  return marks
end

---The legend of a board that draws the marks of kinds.
---@param ctx games.DescribeCtx
---@param kinds games.DiscMarkKind[]
---@return games.LegendEntry[]
function M.legend(ctx, kinds)
  local black = { "black", SEAT_HL.black }
  local entries = {
    {
      sample = {
        grid.sample(ctx, black),
        compose.text(" "),
        grid.sample(ctx, { "white", SEAT_HL.white }),
      },
      meaning = "black and white discs",
    },
    { sample = { grid.sample(ctx, { EMPTY_KEY }) }, meaning = "an empty cell" },
    {
      sample = { grid.sample(ctx, black, Group.LAST_MOVE) },
      meaning = "the last disc, also named beside the board",
    },
  }
  if vim.tbl_contains(kinds, MarkKind.LEGAL) then
    entries[#entries + 1] =
      { sample = { grid.sample(ctx, { LEGAL_KEY }, LEGAL_HL) }, meaning = "a cell you may play" }
  end
  if vim.tbl_contains(kinds, MarkKind.WINNING_LINE) then
    entries[#entries + 1] = {
      sample = { grid.sample(ctx, black, Group.WIN) },
      meaning = "a winning line, also named beside the board",
    }
  end
  return entries
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
  return { cols = notation.letters(size), rows = notation.numbers(size) }
end

---The board, with the notes beside it.
---@param update games.GameUpdate
---@param ctx games.LayoutCtx
---@param props games.DiscBoardProps
---@return games.Node
function M.layout(update, ctx, props)
  ---@type games.DiscView
  local view = update.view
  local legal = grid.cell_index(props.legal or {})
  return board({
    grid = {
      rows = props.size,
      cols = props.size,
      labels = labels(props.size),
      cell = function(r, c)
        local seat = view.cells[r][c]
        if seat ~= "" then
          return { seat, SEAT_HL[seat] }
        end
        return { (legal[r] and legal[r][c]) and LEGAL_KEY or EMPTY_KEY }
      end,
      marks = M.marks(view, props),
    },
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
