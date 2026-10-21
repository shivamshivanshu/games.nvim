local board = require("games.ui.widgets.board")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")

---@alias games.G2048Seat "solver"

---@alias games.G2048Direction "up"|"down"|"left"|"right"

---@class games.G2048View
---@field cells integer[][] rows from the top; 0 for an empty cell
---@field score integer the sum of every merge so far
---@field reached boolean whether a 2048 tile has been made; play goes on after it
---@field movable games.G2048Direction[] the directions that would change the board; empty once over

---@class games.G2048Update: games.GameUpdate
---@field view games.G2048View

---@class games.G2048Tile
---@field row integer 1 at the top
---@field col integer 1 at the left
---@field value integer

---@class games.G2048MovedEvent: games.Event
---@field kind "moved"
---@field by games.G2048Seat
---@field direction games.G2048Direction
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
local GOAL = 2048
local MOVE_KIND = "move"
local GOAL_HL = "GamesWin"
local KEY_HL, VALUE_HL = "GamesKey", "GamesLabel"
local SCORE_LABEL, GOAL_LABEL = "score", tostring(GOAL)
local REACHED, NOT_YET = "reached", "not yet"
---Display columns the status panel reserves: the longer label, a space, and the widest value.
local PANEL_WIDTH = #SCORE_LABEL + 1 + #NOT_YET

---@class (private) games.G2048DirectionKey
---@field key string
---@field direction games.G2048Direction

---Each direction's letter, then its arrow, so the footer pairs them: `h,<Left> left`.
---@type games.G2048DirectionKey[]
local DIRECTION_KEYS = {
  { key = "h", direction = "left" },
  { key = "<Left>", direction = "left" },
  { key = "j", direction = "down" },
  { key = "<Down>", direction = "down" },
  { key = "k", direction = "up" },
  { key = "<Up>", direction = "up" },
  { key = "l", direction = "right" },
  { key = "<Right>", direction = "right" },
}

---@param view games.G2048View
---@return games.GridMark[] every tile of the goal or above
local function marks_for(view)
  local marks = {}
  for row, values in ipairs(view.cells) do
    for col, value in ipairs(values) do
      if value >= GOAL then
        marks[#marks + 1] = { row = row, col = col, hl = GOAL_HL }
      end
    end
  end
  return marks
end

---@param entry games.G2048DirectionKey
---@return games.GameAction
local function move_action(entry)
  return {
    key = entry.key,
    label = entry.direction,
    action = { kind = MOVE_KIND, direction = entry.direction },
    available = function(update)
      return vim.tbl_contains(update.view.movable, entry.direction)
    end,
  }
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
    art = assert(ctx.art, "g2048 draws from an asset pack"),
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv({
            pairs = {
              { SCORE_LABEL, tostring(view.score) },
              { GOAL_LABEL, view.reached and REACHED or NOT_YET },
            },
            key_hl = KEY_HL,
            value_hl = VALUE_HL,
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.tbl_map(move_action, DIRECTION_KEYS)

---@param event games.Event
---@param update games.G2048Update
---@return string|nil
function M.describe_event(event, update)
  local who = players.user_of_seat(update, event.by) or event.by
  if event.kind == "moved" then
    ---@cast event games.G2048MovedEvent
    local gained = event.gained > 0 and (" +%d"):format(event.gained) or ""
    return ("%s moved %s%s"):format(who, event.direction, gained)
  end
  if event.kind == "reached" then
    ---@cast event games.G2048ReachedEvent
    return ("%s made %d"):format(who, event.tile)
  end
  return nil
end

return M
