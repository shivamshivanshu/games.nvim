local actions = require("games.ui.actions")
local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local kv = require("games.ui.widgets.kv")
local tables = require("games.core.tables")

---@alias games.FifteenSeat "solver"

---@class games.FifteenMovable
---@field tile integer
---@field direction games.Direction the way the tile moves into the blank: up moves the tile below it

---@class games.FifteenView
---@field tiles integer[][] rows from the top; 0 is the blank
---@field moves integer slides so far, the score
---@field movable games.FifteenMovable[] tiles that may slide now; empty once over

---@class games.FifteenUpdate: games.GameUpdate
---@field view games.FifteenView

---@class games.FifteenSlidEvent: games.Event
---@field kind "slid"
---@field by games.FifteenSeat
---@field tile integer
---@field direction games.Direction

local CELL_WIDTH = 4
local MAX_TILE = 15
local BLANK = 0
local BLANK_KEY = "blank"
local SLIDE_KIND, MOVE_KIND = "slide", "move"
local MOVABLE_MARKER = "movable"
local MOVABLE_HL = "GamesMovable"
local SOLVED_HL = "GamesWin"
local SOLVED_REASON = "solved"
local MOVES_LABEL = "moves"
---Display columns the status panel reserves: the label, a space, and up to 5 digits.
local PANEL_WIDTH = #MOVES_LABEL + 1 + 5

---@param tile integer
---@param movable boolean
---@return string the asset key: the number, or "movable:7" for a tile that may slide
local function key_for(tile, movable)
  return art.marked_key(tile, movable and MOVABLE_MARKER or nil)
end

---@param view games.FifteenView
---@return table<integer, true>
local function movable_tiles(view)
  return tables.set_of(view.movable, function(entry)
    return entry.tile
  end)
end

---@param update games.FifteenUpdate
---@param movable table<integer, true> the tiles that may slide
---@return games.GridMark[] the movable tiles while playing; every tile once solved
local function marks_for(update, movable)
  local solved = update.result and update.result.reason == SOLVED_REASON
  return grid.marks_where(update.view.tiles, function(tile)
    if solved and tile ~= BLANK then
      return SOLVED_HL
    end
    return movable[tile] and MOVABLE_HL or nil
  end)
end

local M = {}

---@type string[] keys every fifteen asset pack draws: each tile plain and movable, and the blank
M.asset_keys = vim.list_extend(
  vim
    .iter(vim.fn.range(1, MAX_TILE))
    :map(function(tile)
      return { key_for(tile, false), key_for(tile, true) }
    end)
    :flatten()
    :totable(),
  { BLANK_KEY }
)

M.glyph_width = CELL_WIDTH

---@param update games.FifteenUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local movable = movable_tiles(view)
  return board({
    grid = {
      rows = #view.tiles,
      cols = #view.tiles[1],
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        local tile = view.tiles[r][c]
        return { tile == BLANK and BLANK_KEY or key_for(tile, movable[tile] == true) }
      end,
      marks = marks_for(update, movable),
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv.panel({ { MOVES_LABEL, tostring(view.moves) } })
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = SLIDE_KIND,
  prompt = "Slide tile, e.g. 7: ",
}

---@type games.GameAction[]
M.actions = actions.directions({
  kind = MOVE_KIND,
  movable = function(view, direction)
    return vim.iter(view.movable):any(function(movable)
      return movable.direction == direction
    end)
  end,
})

---@type table<games.EventKind, games.Describer>
M.describers = {
  slid = function(event, talk)
    ---@cast event games.FifteenSlidEvent
    return ("%s slid %d %s"):format(talk.name(event.by), event.tile, event.direction)
  end,
}

return M
