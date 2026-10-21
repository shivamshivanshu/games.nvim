local board = require("games.ui.widgets.board")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")

---@alias games.FifteenSeat "solver"

---The way a tile moves into the blank: up moves the tile below it.
---@alias games.FifteenDirection "up"|"down"|"left"|"right"

---@class games.FifteenMovable
---@field tile integer
---@field direction games.FifteenDirection

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
---@field direction games.FifteenDirection

local CELL_WIDTH = 4
local MAX_TILE = 15
local BLANK = 0
local BLANK_KEY = "blank"
local SLIDE_KIND, MOVE_KIND = "slide", "move"
local MOVABLE_HL = "GamesMovable"
local SOLVED_HL = "GamesWin"
local SOLVED_REASON = "solved"
local KEY_HL, VALUE_HL = "GamesKey", "GamesLabel"
local MOVES_LABEL = "moves"
---Display columns the status panel reserves: the label, a space, and up to 5 digits.
local PANEL_WIDTH = #MOVES_LABEL + 1 + 5

---@class (private) games.FifteenDirectionKey
---@field key string
---@field direction games.FifteenDirection

---@type games.FifteenDirectionKey[]
local DIRECTION_KEYS = {
  { key = "h", direction = "left" },
  { key = "j", direction = "down" },
  { key = "k", direction = "up" },
  { key = "l", direction = "right" },
}

---@param tile integer
---@param movable boolean
---@return string the asset key: the number, or "movable:7" for a tile that may slide
local function key_for(tile, movable)
  return movable and ("movable:%d"):format(tile) or tostring(tile)
end

---@param view games.FifteenView
---@return table<integer, true>
local function movable_tiles(view)
  local tiles = {}
  for _, entry in ipairs(view.movable) do
    tiles[entry.tile] = true
  end
  return tiles
end

---@param update games.FifteenUpdate
---@return games.GridMark[] the movable tiles while playing; every tile once solved
local function marks_for(update)
  local view, movable = update.view, movable_tiles(update.view)
  local solved = update.result and update.result.reason == SOLVED_REASON
  local marks = {}
  for row, tiles in ipairs(view.tiles) do
    for col, tile in ipairs(tiles) do
      if solved and tile ~= BLANK then
        marks[#marks + 1] = { row = row, col = col, hl = SOLVED_HL }
      elseif movable[tile] then
        marks[#marks + 1] = { row = row, col = col, hl = MOVABLE_HL }
      end
    end
  end
  return marks
end

---@param entry games.FifteenDirectionKey
---@return games.GameAction
local function move_action(entry)
  return {
    key = entry.key,
    label = entry.direction,
    action = { kind = MOVE_KIND, direction = entry.direction },
    available = function(update)
      return vim.iter(update.view.movable):any(function(movable)
        return movable.direction == entry.direction
      end)
    end,
  }
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
      shade = "plain",
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        local tile = view.tiles[r][c]
        return { tile == BLANK and BLANK_KEY or key_for(tile, movable[tile] == true) }
      end,
      marks = marks_for(update),
    },
    art = assert(ctx.art, "fifteen draws from an asset pack"),
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv({
            pairs = { { MOVES_LABEL, tostring(view.moves) } },
            key_hl = KEY_HL,
            value_hl = VALUE_HL,
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = SLIDE_KIND,
  prompt = "Slide tile, e.g. 7: ",
  parse = function(text)
    return { kind = SLIDE_KIND, tile = text }
  end,
}

---@type games.GameAction[]
M.actions = vim.tbl_map(move_action, DIRECTION_KEYS)

---@param event games.Event
---@param update games.FifteenUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "slid" then
    return nil
  end
  ---@cast event games.FifteenSlidEvent
  local who = players.user_of_seat(update, event.by) or event.by
  return ("%s slid %d %s"):format(who, event.tile, event.direction)
end

return M
