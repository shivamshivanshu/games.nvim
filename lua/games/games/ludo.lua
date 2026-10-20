local board = require("games.ui.widgets.board")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local theme = require("games.ui.theme")

---@alias games.LudoSeat "blue"|"yellow"|"red"|"green"
---@alias games.LudoPhase "roll"|"move"|"finished"

---@class games.LudoView
---@field starts table<games.LudoSeat, integer> each seat in play's start square on the track
---@field progress table<games.LudoSeat, integer[]> each seat in play's 4 tokens: -1 base, 0-50 track, 51-55 home column, 56 home
---@field last_roll integer 0 before the first roll
---@field sixes_in_row integer
---@field phase games.LudoPhase
---@field movable integer[] the viewer's own tokens it may move now
---@field can_roll boolean whether the viewer may roll now

---@class games.LudoUpdate: games.GameUpdate
---@field view games.LudoView

---@class games.LudoRolledEvent: games.Event
---@field kind "rolled"
---@field by games.LudoSeat
---@field value integer

---@class games.LudoPassedEvent: games.Event
---@field kind "passed"
---@field by games.LudoSeat

---@class games.LudoForfeitedEvent: games.Event
---@field kind "forfeited"
---@field by games.LudoSeat

---@class games.LudoMovedEvent: games.Event
---@field kind "moved"
---@field by games.LudoSeat
---@field token integer
---@field from integer
---@field to integer

---@class games.LudoCapturedEvent: games.Event
---@field kind "captured"
---@field by games.LudoSeat
---@field victim games.LudoSeat
---@field token integer

---@alias games.LudoCell {[1]: integer, [2]: integer} row, col

local SIZE = 15
local TOKENS_PER_SEAT = 4
local TRACK_LENGTH = 52
local QUARTER = TRACK_LENGTH / 4
local BASE = -1
local LAST_TRACK = 50
local HOME = 56
local STAR_OFFSET = 8

---The track's first quarter, from the top-left seat's start clockwise to the next start; each
---later quarter is this one turned a quarter clockwise.
---@type games.LudoCell[]
local FIRST_QUARTER = {
  { 7, 2 },
  { 7, 3 },
  { 7, 4 },
  { 7, 5 },
  { 7, 6 },
  { 6, 7 },
  { 5, 7 },
  { 4, 7 },
  { 3, 7 },
  { 2, 7 },
  { 1, 7 },
  { 1, 8 },
  { 1, 9 },
}
---@type games.LudoCell[] the top-left seat's home column, outside in
local FIRST_LANE = { { 8, 2 }, { 8, 3 }, { 8, 4 }, { 8, 5 }, { 8, 6 } }
---@type games.LudoCell[] the top-left seat's base, one spot per token
local FIRST_SPOTS = { { 2, 2 }, { 2, 5 }, { 5, 2 }, { 5, 5 } }
---@type games.LudoCell where the top-left seat's finished tokens stand
local FIRST_GOAL = { 8, 7 }
local YARD_SIZE = 6

local LAST_MOVE_HL = "GamesLastMove"
local PANEL_WIDTH = 14
local MOVABLE_SUFFIX = " ←"

---@param cell games.LudoCell
---@param turns integer quarter turns clockwise
---@return games.LudoCell
local function turned(cell, turns)
  local r, c = cell[1], cell[2]
  for _ = 1, turns do
    r, c = c, SIZE + 1 - r
  end
  return { r, c }
end

---@param r integer
---@param c integer
---@return string
local function cell_id(r, c)
  return r .. "," .. c
end

---@class (private) games.LudoSquare
---@field key string asset key when no token stands on it
---@field quarter? integer whose seat colours it, 0 at the top left, clockwise

---What each square of the board is, with no tokens on it.
---@type table<string, games.LudoSquare>
local SQUARES = (function()
  local squares = {}
  local function put(cell, square)
    squares[cell_id(cell[1], cell[2])] = square
  end
  for quarter = 0, 3 do
    for r = 1, YARD_SIZE do
      for c = 1, YARD_SIZE do
        put(turned({ r, c }, quarter), { key = "yard", quarter = quarter })
      end
    end
    for i, cell in ipairs(FIRST_QUARTER) do
      local offset = i - 1
      local safe = offset == 0 or offset == STAR_OFFSET
      put(
        turned(cell, quarter),
        { key = safe and "safe" or "track", quarter = offset == 0 and quarter or nil }
      )
    end
    for _, cell in ipairs(FIRST_LANE) do
      put(turned(cell, quarter), { key = "lane", quarter = quarter })
    end
    for _, cell in ipairs(FIRST_SPOTS) do
      put(turned(cell, quarter), { key = "spot", quarter = quarter })
    end
    put(turned(FIRST_GOAL, quarter), { key = "goal", quarter = quarter })
  end
  return squares
end)()

---@param start integer
---@return integer quarter of the board the seat starting there sits in
local function quarter_of(start)
  return math.floor(start / QUARTER)
end

---The square a token stands on.
---@param start integer its seat's start square on the track
---@param token integer
---@param progress integer
---@return games.LudoCell
local function cell_of(start, token, progress)
  local quarter = quarter_of(start)
  if progress == BASE then
    return turned(FIRST_SPOTS[token], quarter)
  end
  if progress <= LAST_TRACK then
    local square = (start + progress) % TRACK_LENGTH
    return turned(FIRST_QUARTER[square % QUARTER + 1], quarter_of(square))
  end
  if progress < HOME then
    return turned(FIRST_LANE[progress - LAST_TRACK], quarter)
  end
  return turned(FIRST_GOAL, quarter)
end

---@param update games.LudoUpdate
---@return table<string, table<games.LudoSeat, integer>> tokens per seat on each occupied square
local function occupants(update)
  local view = update.view
  local by_cell = {}
  for seat, tokens in pairs(view.progress) do
    for token, progress in ipairs(tokens) do
      local cell = cell_of(view.starts[seat], token, progress)
      local id = cell_id(cell[1], cell[2])
      by_cell[id] = by_cell[id] or {}
      by_cell[id][seat] = (by_cell[id][seat] or 0) + 1
    end
  end
  return by_cell
end

---@param update games.LudoUpdate
---@param seat games.LudoSeat
---@return string|nil
local function seat_hl(update, seat)
  for i, player in ipairs(update.players) do
    if player.seat == seat then
      return theme.seat_hl(i)
    end
  end
  return nil
end

---@param update games.LudoUpdate
---@return table<integer, games.LudoSeat> the seat in play in each quarter
local function seats_by_quarter(update)
  local by_quarter = {}
  for seat, start in pairs(update.view.starts) do
    by_quarter[quarter_of(start)] = seat
  end
  return by_quarter
end

---@param update games.LudoUpdate
---@return fun(r: integer, c: integer): games.GridCell
local function cells(update)
  local tokens = occupants(update)
  local owners = seats_by_quarter(update)
  return function(r, c)
    local id = cell_id(r, c)
    local here = tokens[id]
    if here then
      local seats = vim.tbl_keys(here)
      if #seats > 1 then
        return { "mixed" }
      end
      local seat = seats[1]
      return { seat .. here[seat], seat_hl(update, seat) }
    end
    local square = SQUARES[id] or { key = "center" }
    local owner = square.quarter and owners[square.quarter]
    return { square.key, owner and seat_hl(update, owner) }
  end
end

---@param update games.LudoUpdate
---@return games.GridMark[] the square the latest token move landed on
local function marks_for(update)
  for i = #update.events, 1, -1 do
    local event = update.events[i]
    local start = update.view.starts[event.by]
    if event.kind == "moved" and start then
      ---@cast event games.LudoMovedEvent
      local cell = cell_of(start, event.token, event.to)
      return { { row = cell[1], col = cell[2], hl = LAST_MOVE_HL } }
    end
  end
  return {}
end

---@param progress integer
---@return string
local function progress_text(progress)
  if progress == BASE then
    return "base"
  end
  if progress == HOME then
    return "home"
  end
  return ("%d/%d"):format(progress, HOME)
end

---@param update games.LudoUpdate
---@param ctx games.LayoutCtx
---@return games.KvPair[]
local function panel_pairs(update, ctx)
  local view = update.view
  local rows = {
    { "roll", view.last_roll > 0 and tostring(view.last_roll) or "-" },
    { "sixes", tostring(view.sixes_in_row) },
    { "phase", view.phase },
  }
  local mine = view.progress[players.seat_of_user(update, ctx.me)]
  for token, progress in ipairs(mine or {}) do
    local movable = vim.tbl_contains(view.movable, token) and MOVABLE_SUFFIX or ""
    rows[#rows + 1] = { tostring(token), progress_text(progress) .. movable }
  end
  return rows
end

local M = {}

---@type integer
M.glyph_width = 2

---@type string[] keys every ludo asset pack draws
M.asset_keys = (function()
  local keys = { "track", "safe", "lane", "spot", "goal", "yard", "center", "mixed" }
  for _, seat in ipairs({ "blue", "yellow", "red", "green" }) do
    for n = 1, TOKENS_PER_SEAT do
      keys[#keys + 1] = seat .. n
    end
  end
  return keys
end)()

---@param update games.LudoUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return board({
    grid = {
      rows = SIZE,
      cols = SIZE,
      shade = "plain",
      cell_width = M.glyph_width,
      cell = cells(update),
      marks = marks_for(update),
    },
    art = assert(ctx.art, "ludo draws from an asset pack"),
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv({
            pairs = panel_pairs(update, ctx),
            key_hl = "GamesKey",
            value_hl = "GamesLabel",
          })
        end,
      },
    },
  }, ctx)
end

---@param token integer
---@return games.GameAction
local function move_action(token)
  return {
    key = tostring(token),
    label = "move",
    action = { kind = "move", token = tostring(token) },
    available = function(update)
      return vim.tbl_contains(update.view.movable, token)
    end,
  }
end

---@type games.GameAction[]
M.actions = {
  {
    key = "r",
    label = "roll",
    action = { kind = "roll" },
    available = function(update)
      return update.view.can_roll
    end,
  },
}
for token = 1, TOKENS_PER_SEAT do
  M.actions[#M.actions + 1] = move_action(token)
end

---@param update games.LudoUpdate
---@param seat games.LudoSeat
---@return string
local function name_of(update, seat)
  return players.user_of_seat(update, seat) or seat
end

---@type table<string, fun(event: games.Event, update: games.LudoUpdate): string>
local DESCRIBE = {
  rolled = function(event, update)
    ---@cast event games.LudoRolledEvent
    return ("%s rolled %d"):format(name_of(update, event.by), event.value)
  end,
  passed = function(event, update)
    return ("%s cannot move"):format(name_of(update, event.by))
  end,
  forfeited = function(event, update)
    return ("%s rolled a third six and loses the turn"):format(name_of(update, event.by))
  end,
  moved = function(event, update)
    ---@cast event games.LudoMovedEvent
    local who = name_of(update, event.by)
    if event.from == BASE then
      return ("%s brought token %d out"):format(who, event.token)
    end
    if event.to == HOME then
      return ("%s brought token %d home"):format(who, event.token)
    end
    return ("%s moved token %d to %s"):format(who, event.token, progress_text(event.to))
  end,
  captured = function(event, update)
    ---@cast event games.LudoCapturedEvent
    return ("%s sent %s's token %d to base"):format(
      name_of(update, event.by),
      name_of(update, event.victim),
      event.token
    )
  end,
}

---@param event games.Event
---@param update games.LudoUpdate
---@return string|nil
function M.describe_event(event, update)
  local describe = DESCRIBE[event.kind]
  return describe and describe(event, update)
end

return M
