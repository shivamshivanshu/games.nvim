local actions = require("games.ui.actions")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local kv = require("games.ui.widgets.kv")

---@alias games.G2048Seat "solver"

---@class games.G2048View
---@field cells integer[][] rows from the top; 0 for an empty cell
---@field score integer the sum of every merge so far
---@field goal integer the tile that marks the game reached
---@field reached boolean whether a goal tile has been made; play goes on after it
---@field movable games.Direction[] the directions that would change the board; empty once over

---@class games.G2048Update: games.GameUpdate
---@field view games.G2048View

---@class games.G2048Tile
---@field row integer 1 at the top
---@field col integer 1 at the left
---@field value integer

---@class games.G2048MovedEvent: games.Event
---@field kind "moved"
---@field by games.G2048Seat
---@field direction games.Direction
---@field gained integer
---@field spawned games.G2048Tile the tile that appeared after the move

---@class games.G2048ReachedEvent: games.Event
---@field kind "reached"
---@field by games.G2048Seat
---@field tile integer

local CELL_WIDTH = 6
local EMPTY = 0
local EMPTY_KEY = "empty"
---2^17, the largest tile a 4x4 board can hold.
local MAX_EXPONENT = 17
local MOVE_KIND = "move"
local GOAL_HL = "GamesWin"
local SCORE_LABEL = "score"
local REACHED, NOT_YET = "reached", "not yet"
---Display columns the status panel reserves: the longer label, a space, and the widest value.
local PANEL_WIDTH = #SCORE_LABEL + 1 + #NOT_YET

---@param view games.G2048View
---@return games.GridMark[] every tile of the goal or above
local function marks_for(view)
  return grid.marks_where(view.cells, function(value)
    return value >= view.goal and GOAL_HL or nil
  end)
end

local M = {}

---@type string[] the asset key of every tile a board can hold: its value as text
M.tile_keys = vim.tbl_map(function(exponent)
  return ("%d"):format(2 ^ exponent)
end, vim.fn.range(1, MAX_EXPONENT))

M.EMPTY_KEY = EMPTY_KEY

---@type string[] keys every g2048 asset pack draws: each tile by its value, and the empty cell
M.asset_keys = vim.list_extend(vim.list_extend({}, M.tile_keys), { EMPTY_KEY })

M.glyph_width = CELL_WIDTH

---@param update games.G2048Update
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = #view.cells,
      cols = #view.cells[1],
      shade = "checker",
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        local value = view.cells[r][c]
        return { value == EMPTY and EMPTY_KEY or tostring(value) }
      end,
      marks = marks_for(view),
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv.panel({
            { SCORE_LABEL, tostring(view.score) },
            { tostring(view.goal), view.reached and REACHED or NOT_YET },
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = actions.directions({
  kind = MOVE_KIND,
  arrows = true,
  movable = function(view, direction)
    return vim.tbl_contains(view.movable, direction)
  end,
})

---@type table<games.EventKind, games.Describer>
M.describers = {
  moved = function(event, talk)
    ---@cast event games.G2048MovedEvent
    local gained = event.gained > 0 and (" +%d"):format(event.gained) or ""
    return ("%s moved %s%s"):format(talk.name(event.by), event.direction, gained)
  end,
  reached = function(event, talk)
    ---@cast event games.G2048ReachedEvent
    return ("%s made %d"):format(talk.name(event.by), event.tile)
  end,
}

return M
