local actions = require("games.ui.actions")
local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local theme = require("games.ui.theme")

local Group = highlights.Group

---@alias games.LudoSeat "blue"|"yellow"|"red"|"green"
---@alias games.LudoPhase "roll"|"move"|"finished"
---@alias games.LudoSquare integer a shared-track square, 0-51 clockwise
---@alias games.LudoLocationKind "base"|"track"|"home_column"|"home"

---@class games.LudoLocation
---@field kind games.LudoLocationKind
---@field square? games.LudoSquare on the track
---@field step? integer 1-5 up the seat's home column

---@class games.LudoLastMove
---@field seat games.LudoSeat
---@field token integer
---@field to games.LudoLocation

---@class games.LudoView
---@field starts table<games.LudoSeat, games.LudoSquare> each seat in play's start square
---@field safe games.LudoSquare[] squares where no capture happens
---@field progress table<games.LudoSeat, integer[]> each seat in play's 4 tokens' steps from base
---@field locations table<games.LudoSeat, games.LudoLocation[]> where each seat in play's 4 tokens stand
---@field last_roll integer 0 before the first roll
---@field sixes_in_row integer
---@field last? games.LudoLastMove the move the latest action made; nil after a roll
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
---@field from games.LudoLocation
---@field to games.LudoLocation

---@class games.LudoCapturedEvent: games.Event
---@field kind "captured"
---@field by games.LudoSeat
---@field victim games.LudoSeat
---@field token integer

---@alias games.LudoCell {[1]: integer, [2]: integer} row, col

local SIZE = 15
local TOKENS_PER_SEAT = 4
local MAX_MIXED_COUNT = 9
---@type games.LudoSeat[] every seat, in the shared seat-colour order
local SEATS = { "blue", "yellow", "red", "green" }
local ARMS = 4
local YARD_SIZE = 6

---The drawn loop's first quarter, clockwise from the top-left arm's start square; each later
---quarter is this one turned a quarter clockwise.
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
---@type games.LudoCell[] the top-left arm's home column, outside in
local FIRST_LANE = { { 8, 2 }, { 8, 3 }, { 8, 4 }, { 8, 5 }, { 8, 6 } }
---@type games.LudoCell[] the top-left arm's base, one spot per token
local FIRST_SPOTS = { { 2, 2 }, { 2, 5 }, { 5, 2 }, { 5, 5 } }
---@type games.LudoCell where the top-left arm's finished tokens stand
local FIRST_GOAL = { 8, 7 }

local MOVABLE_KEY = "movable"
local TRACK_KEY = "track"
local PANEL_WIDTH = 16

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

---@param row integer
---@param col integer
---@return integer
local function key_at(row, col)
  return (row - 1) * SIZE + col
end

---@param cell games.LudoCell
---@return integer
local function cell_key(cell)
  return key_at(cell[1], cell[2])
end

---@param cells games.LudoCell[]
---@param turns integer
---@return games.LudoCell[]
local function all_turned(cells, turns)
  return vim.tbl_map(function(cell)
    return turned(cell, turns)
  end, cells)
end

---@class (private) games.LudoArm the drawing of one corner: base, home column and goal
---@field start games.LudoCell the loop square a seat starting here starts on
---@field spots games.LudoCell[]
---@field lane games.LudoCell[]
---@field goal games.LudoCell
---@field yard games.LudoCell[]

---@type games.LudoArm[]
local ARM_DRAWINGS = vim
  .iter(vim.fn.range(0, ARMS - 1))
  :map(function(turns)
    local yard = {}
    for r = 1, YARD_SIZE do
      for c = 1, YARD_SIZE do
        yard[#yard + 1] = turned({ r, c }, turns)
      end
    end
    return {
      start = turned(FIRST_QUARTER[1], turns),
      spots = all_turned(FIRST_SPOTS, turns),
      lane = all_turned(FIRST_LANE, turns),
      goal = turned(FIRST_GOAL, turns),
      yard = yard,
    }
  end)
  :totable()

---@type games.LudoCell[] the drawn track, by square from 0
local LOOP = vim
  .iter(vim.fn.range(0, ARMS - 1))
  :map(function(turns)
    return all_turned(FIRST_QUARTER, turns)
  end)
  :flatten()
  :totable()

---@param square games.LudoSquare
---@return games.LudoCell
local function loop_cell(square)
  return LOOP[square + 1]
end

---@type table<games.LudoSquare, games.LudoArm> each arm by the square it starts on
local ARM_AT_START = (function()
  local square_at = {}
  for i, cell in ipairs(LOOP) do
    square_at[cell_key(cell)] = i - 1
  end
  local arms = {}
  for _, arm in ipairs(ARM_DRAWINGS) do
    arms[square_at[cell_key(arm.start)]] = arm
  end
  return arms
end)()

---@param start games.LudoSquare
---@return games.LudoArm the arm drawn at the start square
local function arm_at(start)
  return assert(ARM_AT_START[start], "no arm starts at square " .. start)
end

---@type table<integer, string> the asset key of each drawn cell but the centre's, whatever the view
local STATIC_SQUARES = (function()
  local squares = {}
  for _, cell in ipairs(LOOP) do
    squares[cell_key(cell)] = TRACK_KEY
  end
  for _, arm in ipairs(ARM_DRAWINGS) do
    for _, cell in ipairs(arm.yard) do
      squares[cell_key(cell)] = "yard"
    end
    for _, cell in ipairs(arm.spots) do
      squares[cell_key(cell)] = "spot"
    end
    for _, cell in ipairs(arm.lane) do
      squares[cell_key(cell)] = "lane"
    end
    squares[cell_key(arm.goal)] = "goal"
  end
  return squares
end)()

---@param arm games.LudoArm
---@param token integer
---@param location games.LudoLocation
---@return games.LudoCell
local function cell_of(arm, token, location)
  if location.kind == "base" then
    return arm.spots[token]
  end
  if location.kind == "track" then
    return loop_cell(assert(location.square))
  end
  if location.kind == "home_column" then
    return arm.lane[assert(location.step)]
  end
  return arm.goal
end

---@class (private) games.LudoBoard what the view puts where
---@field tokens table<integer, table<games.LudoSeat, integer>> tokens per seat on each occupied cell
---@field owners table<integer, games.LudoSeat> the seat of each cell of a seat in play's arm
---@field safe table<integer, true> the track cells where no capture happens

---@param view games.LudoView
---@return games.LudoBoard
local function board_of(view)
  local safe, owners, tokens = {}, {}, {}
  for _, square in ipairs(view.safe) do
    safe[cell_key(loop_cell(square))] = true
  end
  for seat, start in pairs(view.starts) do
    local arm = arm_at(start)
    local owned = { arm.start, arm.goal }
    vim.list_extend(owned, arm.spots)
    vim.list_extend(owned, arm.lane)
    for _, cell in ipairs(owned) do
      owners[cell_key(cell)] = seat
    end
    for token, location in ipairs(view.locations[seat]) do
      local key = cell_key(cell_of(arm, token, location))
      tokens[key] = tokens[key] or {}
      tokens[key][seat] = (tokens[key][seat] or 0) + 1
    end
  end
  return { tokens = tokens, owners = owners, safe = safe }
end

---@param here table<games.LudoSeat, integer>
---@return integer
local function total(here)
  return vim.iter(vim.tbl_values(here)):fold(0, function(sum, n)
    return sum + n
  end)
end

---@param update games.LudoUpdate
---@return fun(r: integer, c: integer): games.GridCell
local function cells(update)
  local drawn = board_of(update.view)
  return function(r, c)
    local key = key_at(r, c)
    local here = drawn.tokens[key]
    if here then
      local seats = vim.tbl_keys(here)
      if #seats > 1 then
        return { "mixed" .. math.min(total(here), MAX_MIXED_COUNT) }
      end
      local seat = seats[1]
      return { seat .. here[seat], theme.seat_hl_of(update, seat) }
    end
    local square = STATIC_SQUARES[key]
    local seat = drawn.owners[key]
    return {
      (square == TRACK_KEY and drawn.safe[key]) and "safe" or square or "center",
      seat and theme.seat_hl_of(update, seat),
    }
  end
end

---@param view games.LudoView
---@return games.GridMark[] the square the latest token move landed on
local function marks_for(view)
  local last = view.last
  if not last then
    return {}
  end
  local cell = cell_of(arm_at(view.starts[last.seat]), last.token, last.to)
  return { grid.mark(cell[1], cell[2], Group.LAST_MOVE) }
end

---@param location games.LudoLocation
---@param steps integer the token's steps from base
---@return string
local function location_text(location, steps)
  if location.kind == "track" then
    return ("track %d"):format(steps)
  end
  if location.kind == "home_column" then
    return ("lane %d"):format(location.step)
  end
  return location.kind
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
  local seat = players.seat_of_user(update, ctx.me)
  local mine = seat and view.locations[seat]
  local mark = art.glyph(ctx.art --[[@as games.ArtSet]], MOVABLE_KEY, ctx.ambiwidth)
  for token, location in ipairs(mine or {}) do
    local movable = vim.tbl_contains(view.movable, token) and mark or ""
    rows[#rows + 1] =
      { tostring(token), location_text(location, view.progress[seat][token]) .. movable }
  end
  return rows
end

local M = {}

---@type integer
M.glyph_width = 2
M.TOKENS_PER_SEAT = TOKENS_PER_SEAT
M.MAX_MIXED_COUNT = MAX_MIXED_COUNT
M.SEATS = SEATS

---@type string[] keys every ludo asset pack draws
M.asset_keys = (function()
  local keys = { TRACK_KEY, "safe", "lane", "spot", "goal", "yard", "center", MOVABLE_KEY }
  for _, seat in ipairs(SEATS) do
    for n = 1, TOKENS_PER_SEAT do
      keys[#keys + 1] = seat .. n
    end
  end
  for n = 2, MAX_MIXED_COUNT do
    keys[#keys + 1] = "mixed" .. n
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
      cell_width = M.glyph_width,
      cell = cells(update),
      marks = marks_for(update.view),
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv.panel(panel_pairs(update, ctx))
        end,
      },
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.list_extend(
  {
    {
      key = "r",
      label = "roll",
      action = { kind = "roll" },
      available = function(update)
        return update.view.can_roll
      end,
    },
  },
  actions.per_value({
    kind = "move",
    field = "token",
    values = vim.fn.range(1, TOKENS_PER_SEAT),
    legal = function(view)
      return view.movable
    end,
  })
)

---@type table<games.EventKind, games.Describer>
M.describers = {
  rolled = function(event, talk)
    ---@cast event games.LudoRolledEvent
    return ("%s rolled %d"):format(talk.name(event.by), event.value)
  end,
  passed = function(event, talk)
    return ("%s cannot move"):format(talk.name(event.by))
  end,
  forfeited = function(event, talk)
    return ("%s rolled a third six and loses the turn"):format(talk.name(event.by))
  end,
  moved = function(event, talk)
    ---@cast event games.LudoMovedEvent
    local who = talk.name(event.by)
    if event.from.kind == "base" then
      return ("%s brought token %d out"):format(who, event.token)
    end
    if event.to.kind == "home" then
      return ("%s brought token %d home"):format(who, event.token)
    end
    if event.to.kind == "home_column" then
      return ("%s moved token %d to lane %d"):format(who, event.token, event.to.step)
    end
    return ("%s moved token %d to square %d"):format(who, event.token, event.to.square)
  end,
  captured = function(event, talk)
    ---@cast event games.LudoCapturedEvent
    return ("%s sent %s's token %d to base"):format(
      talk.name(event.by),
      talk.name(event.victim),
      event.token
    )
  end,
}

return M
