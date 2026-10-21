local actions = require("games.ui.actions")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local notation = require("games.ui.notation")
local theme = require("games.ui.theme")

local Group = highlights.Group

---@alias games.TicTacToeSeat "x"|"o"

---@class games.TicTacToeView
---@field cells (games.TicTacToeSeat|"")[] each cell's owner in phone order, 1 at the top-left; "" when empty
---@field last? games.Cell the cell placed last
---@field winning_lines games.Cell[][] the cells of each completed line
---@field legal_cells integer[] the viewer's own: empty unless it is to move

---@class games.TicTacToeUpdate: games.GameUpdate
---@field view games.TicTacToeView

---@class games.TicTacToePlacedEvent: games.Event
---@field kind "placed"
---@field by games.TicTacToeSeat
---@field cell integer

local SIDE = 3
local CELLS = SIDE * SIDE
local CELL_WIDTH = 3
local PLACE_KIND = "place"
local SEAT_KEYS = { "x", "o" }

local EMPTY_HL = Group.MUTED

---@param row integer
---@param col integer
---@return integer
local function cell_at(row, col)
  return (row - 1) * SIDE + col
end

---An empty cell draws its own number, the key that takes it.
---@param update games.TicTacToeUpdate
---@param cell integer
---@return games.GridCell
local function cell_for(update, cell)
  local owner = update.view.cells[cell]
  if owner == "" then
    return { tostring(cell), EMPTY_HL }
  end
  return { owner, theme.seat_hl_of(update, owner) }
end

---@param view games.TicTacToeView
---@return games.GridMark[] the last move, then the winning lines over it
local function marks_for(view)
  local marks = {}
  if view.last then
    marks[1] = grid.mark(view.last.row, view.last.col, Group.LAST_MOVE)
  end
  for _, line in ipairs(view.winning_lines) do
    vim.list_extend(marks, grid.cell_marks(line, Group.WIN))
  end
  return marks
end

local M = {}

---@type string[] keys every tictactoe asset pack draws: the seats' marks and each empty cell's number
M.asset_keys = vim.list_extend(vim.deepcopy(SEAT_KEYS), notation.numbers(CELLS))

M.glyph_width = CELL_WIDTH

---@param update games.TicTacToeUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return board({
    grid = {
      rows = SIDE,
      cols = SIDE,
      blocks = 1,
      glyphs = ctx.glyphs,
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        return cell_for(update, cell_at(r, c))
      end,
      marks = marks_for(update.view),
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = actions.per_value({
  kind = PLACE_KIND,
  field = "cell",
  values = vim.fn.range(1, CELLS),
  legal = function(view)
    return view.legal_cells
  end,
})

---@type table<games.EventKind, games.Describer>
M.describers = {
  placed = function(event, talk)
    ---@cast event games.TicTacToePlacedEvent
    return ("%s took cell %d"):format(talk.name(event.by), event.cell)
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local x = { SEAT_KEYS[1], theme.seat_hl(1) }
  return {
    {
      sample = {
        grid.sample(ctx, x),
        compose.text(" "),
        grid.sample(ctx, { SEAT_KEYS[2], theme.seat_hl(2) }),
      },
      meaning = "each player's mark, in their colour in the header",
    },
    {
      sample = { grid.sample(ctx, { "5", EMPTY_HL }) },
      meaning = "an empty cell: its number takes it",
    },
    { sample = { grid.sample(ctx, x, Group.LAST_MOVE) }, meaning = "the last move" },
    { sample = { grid.sample(ctx, x, Group.WIN) }, meaning = "a winning line" },
  }
end

return M
