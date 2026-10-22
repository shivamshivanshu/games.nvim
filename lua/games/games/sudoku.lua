local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local notation = require("games.ui.notation")
local theme = require("games.ui.theme")

local Group = highlights.Group

---@alias games.SudokuSeat "solver"

---@class games.SudokuView
---@field grid integer[] 81 digits in row-major order, row 1 at the top; 0 when empty
---@field given boolean[] the puzzle's own cells, which cannot change
---@field conflicts boolean[] filled cells whose digit repeats in their row, column or box
---@field solution integer[] every cell's digit once the game is over; empty until then

---@class games.SudokuUpdate: games.GameUpdate
---@field view games.SudokuView

---@class games.SudokuCellSetEvent: games.Event
---@field kind "cell_set"
---@field by games.SudokuSeat
---@field row integer
---@field col integer
---@field digit integer 0 when the cell was cleared

local SIZE = 9
local CELL_WIDTH = 3
local BOX_SIZE = 3
local EMPTY_KEY = "empty"
---@enum games.SudokuMarker drawn into the glyph, so a cell's state never rests on colour alone
local Marker = { CONFLICT = "conflict", REVEALED = "revealed" }
local SET_KIND = "set"
local CLEAR_KIND = "clear"
local SAMPLE_DIGIT = 5
---@type games.SudokuSeat
local SOLVER = "solver"

local REVEALED_FG_HL = Group.MUTED
local EMPTY_HL = Group.MUTED
local CONFLICT_HL = Group.CHECK
local REVEALED_HL = Group.MISS

---@param row integer
---@param col integer
---@return integer
local function index(row, col)
  return (row - 1) * SIZE + col
end

---What one render of the board works out once, for every cell.
---@class (private) games.SudokuShown
---@field over boolean
---@field corrected table<integer, integer> by cell index, the solution's digit where it differs from the grid
---@field solver_hl games.Hl|nil the solver's seat colour, for the digits they entered

---@param update games.SudokuUpdate
---@return games.SudokuShown
local function shown_of(update)
  local view = update.view
  local corrected = {}
  for i, digit in ipairs(view.solution) do
    if digit ~= view.grid[i] then
      corrected[i] = digit
    end
  end
  return {
    over = update.result ~= nil,
    corrected = corrected,
    solver_hl = theme.seat_hl_of(update, SOLVER),
  }
end

---@param view games.SudokuView
---@param i integer
---@param shown games.SudokuShown
---@return games.GridCell
local function cell_for(view, i, shown)
  local correction = shown.corrected[i]
  if correction then
    return { art.marked_key(correction, Marker.REVEALED), REVEALED_FG_HL }
  end
  local digit = view.grid[i]
  if digit == 0 then
    return { EMPTY_KEY, EMPTY_HL }
  end
  local marker = not shown.over and view.conflicts[i] and Marker.CONFLICT or nil
  return { art.marked_key(digit, marker), view.given[i] and Group.GIVEN or shown.solver_hl }
end

---@param view games.SudokuView
---@param shown games.SudokuShown
---@return games.GridMark[] while playing, the conflicting cells; once over, the cells the solution corrects
local function marks_for(view, shown)
  local marks = {}
  for row = 1, SIZE do
    for col = 1, SIZE do
      local i = index(row, col)
      if shown.over and shown.corrected[i] then
        marks[#marks + 1] = grid.mark(row, col, REVEALED_HL)
      elseif not shown.over and view.conflicts[i] then
        marks[#marks + 1] = grid.mark(row, col, CONFLICT_HL)
      end
    end
  end
  return marks
end

local M = {}

---@type string[] keys every sudoku asset pack draws: each digit plain and per marker, and the empty cell
M.asset_keys = vim.list_extend(
  vim
    .iter(vim.fn.range(1, SIZE))
    :map(function(digit)
      return {
        art.marked_key(digit),
        art.marked_key(digit, Marker.CONFLICT),
        art.marked_key(digit, Marker.REVEALED),
      }
    end)
    :flatten()
    :totable(),
  { EMPTY_KEY }
)

M.glyph_width = CELL_WIDTH

---@param update games.SudokuUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local shown = shown_of(update)
  local labels = notation.numbers(SIZE)
  return board({
    grid = {
      rows = SIZE,
      cols = SIZE,
      blocks = BOX_SIZE,
      cell_width = CELL_WIDTH,
      glyphs = ctx.glyphs,
      labels = { rows = labels, cols = labels },
      cell = function(r, c)
        return cell_for(view, index(r, c), shown)
      end,
      marks = marks_for(view, shown),
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = SET_KIND,
  prompt = "Set, e.g. 53 7: ",
}

---@type games.GameAction[]
M.actions = {
  { key = "x", label = "clear", input = { kind = CLEAR_KIND, prompt = "Clear, e.g. 53: " } },
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  cell_set = function(event, talk)
    ---@cast event games.SudokuCellSetEvent
    local who = talk.name(event.by)
    if event.digit == 0 then
      return ("%s cleared row %d, column %d"):format(who, event.row, event.col)
    end
    return ("%s set row %d, column %d to %d"):format(who, event.row, event.col, event.digit)
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local entered = theme.seat_hl(1)
  return {
    grid.legend_entry(
      ctx,
      { art.marked_key(SAMPLE_DIGIT), Group.GIVEN },
      "a given: part of the puzzle"
    ),
    grid.legend_entry(
      ctx,
      { art.marked_key(SAMPLE_DIGIT), entered },
      "a digit you entered, in your colour"
    ),
    grid.legend_entry(
      ctx,
      { art.marked_key(SAMPLE_DIGIT, Marker.CONFLICT), entered },
      "a conflict: the digit repeats in its row, column or box",
      CONFLICT_HL
    ),
    grid.legend_entry(ctx, { EMPTY_KEY, EMPTY_HL }, "an empty cell"),
    grid.legend_entry(
      ctx,
      { art.marked_key(SAMPLE_DIGIT, Marker.REVEALED), REVEALED_FG_HL },
      "once the game is over: the solution's digit, where yours was wrong or missing",
      REVEALED_HL
    ),
  }
end

return M
