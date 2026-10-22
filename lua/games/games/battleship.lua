local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local cursor = require("games.ui.cursor")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local notation = require("games.ui.notation")
local players = require("games.core.players")
local text_util = require("games.ui.text")

local Group = highlights.Group

---@alias games.BattleshipSeat "p1"|"p2"

---@alias games.BattleshipPhase "setup"|"battle"|"over"

---What a square shows its viewer; the opponent's ships show as "ship" on the target only once over.
---@alias games.BattleshipTile "empty"|"ship"|"miss"|"hit"|"sunk"

---@alias games.BattleshipShipName "carrier"|"battleship"|"cruiser"|"submarine"|"destroyer"

---@alias games.BattleshipShipState "unplaced"|"afloat"|"sunk"

---@alias games.BattleshipShotOutcome "miss"|"hit"|"sunk"

---@class games.BattleshipShipSpec
---@field name games.BattleshipShipName
---@field size integer

---@class games.BattleshipShipStatus
---@field name games.BattleshipShipName
---@field size integer
---@field state games.BattleshipShipState the opponent's ships are afloat or sunk

---@class games.BattleshipAnchors
---@field across games.Cell[] where the next ship's left end may go
---@field down games.Cell[] where its top end may go

---@class games.BattleshipView
---@field phase games.BattleshipPhase
---@field own games.BattleshipTile[][] the viewer's waters, rows from the top
---@field target games.BattleshipTile[][] the opponent's, as the viewer's shots found them
---@field fleet games.BattleshipShipStatus[] the viewer's ships, in placing order
---@field enemy games.BattleshipShipStatus[] the opponent's ships, in the same order
---@field next? games.BattleshipShipSpec the viewer's ship to place now
---@field anchors games.BattleshipAnchors the viewer's own; empty unless placing
---@field targets games.Cell[] the viewer's own: where it may fire; empty unless to fire
---@field last? games.BattleshipShot the latest shot of either seat

---@class games.BattleshipUpdate: games.GameUpdate
---@field view games.BattleshipView

---@class games.BattleshipReadyEvent: games.Event
---@field kind "ready"
---@field by games.BattleshipSeat

---@class games.BattleshipShot
---@field by games.BattleshipSeat
---@field cell games.Cell
---@field outcome games.BattleshipShotOutcome
---@field ship? games.BattleshipShipName the ship it sank

---@class games.BattleshipShotEvent: games.BattleshipShot, games.Event
---@field kind "shot"

local SIZE = 10
---@type games.CursorBounds
local BOUNDS = { rows = SIZE, cols = SIZE }
local TITLES = { "your fleet", "target" }
local PLACE_KIND = { across = "place_across", down = "place_down" }
local FIRE_KIND, RANDOM_KIND = "fire", "random"
local BERTH_KEY, BLOCKED_KEY, AIM_KEY = "berth", "blocked", "aim"
---@type table<games.BattleshipTile, games.Hl>
local TILE_HL = {
  empty = Group.MUTED,
  ship = Group.LABEL,
  miss = Group.MUTED,
  hit = Group.CARD_RED,
  sunk = Group.CARD_RED,
}
local BERTH_HL, BLOCKED_HL, AIM_HL, LAST_HL =
  Group.MOVABLE, Group.CHECK, Group.MOVABLE, Group.LAST_MOVE
local VALUE_HL = Group.LABEL
local SHIP_COLUMN, STATE_COLUMN = 13, 9
---Display columns the panel reserves: a ship's name and size, the viewer's state of it and the
---opponent's, which is never "unplaced"; its widest status line, "waiting for their fleet", fits.
local PANEL_WIDTH = SHIP_COLUMN + STATE_COLUMN + #"theirs"

---@type games.GridLabels
local LABELS = { cols = notation.letters(SIZE), rows = notation.numbers(SIZE) }

---@param cell games.GridPos
---@return string as typed, the column's letter then the row's number, e.g. c5
local function cell_name(cell)
  return text_util.ALPHABET:sub(cell.col, cell.col) .. cell.row
end

---@param update games.GameUpdate
---@return boolean
local function placing(update)
  return update.view.next ~= nil
end

---@param update games.GameUpdate
---@return boolean
local function firing(update)
  return #update.view.targets > 0
end

---@param update games.GameUpdate
---@return boolean
local function aiming(update)
  return placing(update) or firing(update)
end

---@param squares games.Cell[]
---@param cell games.GridPos
---@return boolean
local function contains(squares, cell)
  return vim.iter(squares):any(function(square)
    return square.row == cell.row and square.col == cell.col
  end)
end

---@param view games.BattleshipView
---@param at games.GridCursor
---@return boolean whether the next ship may lie from the cursor as it does
local function berthable(view, at)
  return contains(view.anchors[at.orientation], at)
end

---@param view games.BattleshipView
---@param at games.GridPos
---@return boolean whether the viewer may fire at the cursor's cell
local function open_water(view, at)
  return contains(view.targets, at)
end

---@param tiles games.BattleshipTile[][]
---@param overlay table<integer, table<integer, string>> row -> col -> key drawn over the tile
---@return fun(r: integer, c: integer): games.GridCell
local function cells(tiles, overlay)
  return function(r, c)
    local key = overlay[r] and overlay[r][c]
    local tile = tiles[r][c]
    return { key or tile, not key and TILE_HL[tile] or nil }
  end
end

---@class (private) games.BattleshipLayer
---@field overlay table<integer, table<integer, string>>
---@field marks games.GridMark[]

---@return games.BattleshipLayer
local function layer()
  return { overlay = {}, marks = {} }
end

---@param into games.BattleshipLayer
---@param cell games.GridPos
---@param key string|nil drawn over the tile; nil keeps the tile
---@param hl games.Hl
local function mark(into, cell, key, hl)
  if key then
    into.overlay[cell.row] = into.overlay[cell.row] or {}
    into.overlay[cell.row][cell.col] = key
  end
  into.marks[#into.marks + 1] = grid.mark(cell.row, cell.col, hl)
end

---What each board draws over its tiles: the latest shot, then the ship being placed or the aim.
---@param update games.BattleshipUpdate
---@param ctx games.LayoutCtx
---@return games.BattleshipLayer own
---@return games.BattleshipLayer target
local function layers(update, ctx)
  local view = update.view
  local own, target = layer(), layer()
  local last = view.last
  if last then
    local mine = last.by == players.seat_of_user(update, ctx.me)
    mark(mine and target or own, last.cell, nil, LAST_HL)
  end
  local at = cursor.of(ctx)
  if view.next then
    local legal = berthable(view, at)
    for _, cell in ipairs(cursor.footprint(at, view.next.size, BOUNDS)) do
      mark(own, cell, legal and BERTH_KEY or BLOCKED_KEY, legal and BERTH_HL or BLOCKED_HL)
    end
  elseif firing(update) then
    local open = open_water(view, at)
    mark(target, at, open and AIM_KEY or BLOCKED_KEY, open and AIM_HL or BLOCKED_HL)
  end
  return own, target
end

---@param tiles games.BattleshipTile[][]
---@param drawn games.BattleshipLayer
---@return games.GridProps
local function grid_of(tiles, drawn)
  return {
    rows = SIZE,
    cols = SIZE,
    labels = LABELS,
    cell = cells(tiles, drawn.overlay),
    marks = drawn.marks,
  }
end

---@param update games.BattleshipUpdate
---@param ctx games.LayoutCtx
---@return string[] what the viewer does now, or waits on, in words
local function status_lines(update, ctx)
  local view = update.view
  local at = cursor.of(ctx)
  if view.next then
    local where = ("%s at %s"):format(at.orientation, cell_name(at))
    return {
      ("place %s %d"):format(view.next.name, view.next.size),
      berthable(view, at) and where or (where .. ": blocked"),
    }
  end
  if players.is_to_act(update, ctx.me) then
    local where = "at " .. cell_name(at)
    return { "your shot", open_water(view, at) and where or (where .. ": fired") }
  end
  if view.phase == "over" then
    return { "game over", "" }
  end
  return { view.phase == "setup" and "waiting for their fleet" or "their shot", "" }
end

---@param update games.BattleshipUpdate
---@param ctx games.LayoutCtx
---@return string[] the latest shot in words, none before the first
local function last_lines(update, ctx)
  local last = update.view.last
  if not last then
    return {}
  end
  local whose = last.by == players.seat_of_user(update, ctx.me) and "your" or "their"
  local found = last.ship and ("sank the " .. last.ship) or last.outcome
  return { ("last: %s shot at %s"):format(whose, cell_name(last.cell)), found }
end

---@param update games.BattleshipUpdate
---@param ctx games.LayoutCtx
---@return games.Node
local function panel(update, ctx)
  local view = update.view
  local function row(ship, mine, theirs, hl)
    return compose.row({
      compose.text(text_util.pad_to_width(ship, SHIP_COLUMN), hl),
      compose.text(text_util.pad_to_width(mine, STATE_COLUMN), hl),
      compose.text(theirs, hl),
    })
  end
  local status = status_lines(update, ctx)
  local lines = {
    compose.text(status[1], Group.KEY),
    compose.text(status[2], VALUE_HL),
    row("ship", "yours", "theirs", Group.MUTED),
  }
  for i, ship in ipairs(view.fleet) do
    local name = ("%s %d"):format(ship.name, ship.size)
    lines[#lines + 1] = row(name, ship.state, view.enemy[i].state, VALUE_HL)
  end
  for _, line in ipairs(last_lines(update, ctx)) do
    lines[#lines + 1] = compose.text(line, Group.MUTED)
  end
  return compose.col(lines)
end

local M = {}

---@type string[] keys every battleship asset pack draws: each tile; the ship being placed where it
---fits and the aim where a shot may go; and either where it may not
M.asset_keys = { "empty", "ship", "miss", "hit", "sunk", BERTH_KEY, BLOCKED_KEY, AIM_KEY }

---@param update games.BattleshipUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local own, target = layers(update, ctx)
  return board({
    grid = grid_of(view.own, own),
    beside = { grid_of(view.target, target) },
    titles = TITLES,
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return panel(update, ctx)
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = FIRE_KIND,
  prompt = "Fire, e.g. c5: ",
  available = firing,
}

---@type games.GameAction[]
M.actions = vim.list_extend(cursor.moves(BOUNDS, aiming), {
  cursor.rotate(placing),
  {
    key = "<CR>",
    label = function(update)
      return placing(update) and "place" or "fire"
    end,
    act = function(update, view)
      local at = cursor.of(view)
      if placing(update) then
        return { kind = PLACE_KIND[at.orientation], cell = cell_name(at) }
      end
      return open_water(update.view, at) and { kind = FIRE_KIND, cell = cell_name(at) } or nil
    end,
  },
  { key = "f", label = "fill", action = { kind = RANDOM_KIND }, available = placing },
})

---@type table<games.EventKind, games.Describer>
M.describers = {
  ready = function(event, talk)
    ---@cast event games.BattleshipReadyEvent
    return ("%s's fleet is ready"):format(talk.name(event.by))
  end,
  shot = function(event, talk)
    ---@cast event games.BattleshipShotEvent
    local found = event.ship and ("sank the " .. event.ship) or event.outcome
    return ("%s fired at %s: %s"):format(talk.name(event.by), cell_name(event.cell), found)
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function tile(name, meaning)
    return { sample = { grid.sample(ctx, { name, TILE_HL[name] }) }, meaning = meaning }
  end
  local function overlay(key, hl, meaning)
    return { sample = { grid.sample(ctx, { key }, hl) }, meaning = meaning }
  end
  return {
    tile("empty", "open water"),
    tile("ship", "a ship"),
    tile("miss", "a shot that missed"),
    tile("hit", "a hit"),
    tile("sunk", "a ship sunk"),
    { sample = { grid.sample(ctx, { "hit", TILE_HL.hit }, LAST_HL) }, meaning = "the latest shot" },
    overlay(BERTH_KEY, BERTH_HL, "the ship you are placing, where it fits"),
    overlay(AIM_KEY, AIM_HL, "your aim, at open water"),
    overlay(BLOCKED_KEY, BLOCKED_HL, "the ship does not fit there, or that cell was fired at"),
  }
end

return M
