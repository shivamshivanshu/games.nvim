local board = require("games.ui.widgets.board")
local players = require("games.core.players")
local theme = require("games.ui.theme")

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
local BOX_SIZE = 3
local LABELS = "123456789"
local EMPTY_KEY = "empty"
local SET_KIND = "set"
local SOLVER_SEAT_INDEX = 1

local GIVEN_HL = "GamesMuted"
local EMPTY_HL = "GamesMuted"
local CONFLICT_HL = "GamesCheck"
local REVEALED_HL = "GamesMiss"

---@param row integer
---@param col integer
---@return integer
local function index(row, col)
  return (row - 1) * SIZE + col
end

---@param view games.SudokuView
---@param i integer
---@return integer|nil the solution's digit where it differs from the grid, once the game is over
local function revealed(view, i)
  local digit = view.solution[i]
  if digit ~= nil and digit ~= view.grid[i] then
    return digit
  end
end

---@param view games.SudokuView
---@param i integer
---@return games.GridCell
local function cell_for(view, i)
  local correction = revealed(view, i)
  local digit = correction or view.grid[i]
  if digit == 0 then
    return { EMPTY_KEY, EMPTY_HL }
  end
  local muted = view.given[i] or correction ~= nil
  return { tostring(digit), muted and GIVEN_HL or theme.seat_hl(SOLVER_SEAT_INDEX) }
end

---@param view games.SudokuView
---@return games.GridMark[] while playing, the conflicting cells; once over, the cells the solution corrects
local function marks_for(view)
  local over = #view.solution > 0
  local marks = {}
  for row = 1, SIZE do
    for col = 1, SIZE do
      local i = index(row, col)
      if over and revealed(view, i) then
        marks[#marks + 1] = { row = row, col = col, hl = REVEALED_HL }
      elseif not over and view.conflicts[i] then
        marks[#marks + 1] = { row = row, col = col, hl = CONFLICT_HL }
      end
    end
  end
  return marks
end

local M = {}

---@type string[] keys every sudoku asset pack draws: each digit and the empty cell
M.asset_keys = vim.list_extend(vim.tbl_map(tostring, vim.fn.range(1, SIZE)), { EMPTY_KEY })

---@param update games.SudokuUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = SIZE,
      cols = SIZE,
      shade = "plain",
      blocks = BOX_SIZE,
      glyphs = ctx.glyphs,
      labels = { rows = LABELS, cols = LABELS },
      cell = function(r, c)
        return cell_for(view, index(r, c))
      end,
      marks = marks_for(view),
    },
    art = assert(ctx.art, "sudoku draws from an asset pack"),
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = SET_KIND,
  prompt = "Set, e.g. 53 7: ",
  parse = function(text)
    return { kind = SET_KIND, text = text }
  end,
}

---@param event games.Event
---@param update games.SudokuUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "cell_set" then
    return nil
  end
  ---@cast event games.SudokuCellSetEvent
  local who = players.user_of_seat(update, event.by) or event.by
  if event.digit == 0 then
    return ("%s cleared row %d, column %d"):format(who, event.row, event.col)
  end
  return ("%s set row %d, column %d to %d"):format(who, event.row, event.col, event.digit)
end

return M
