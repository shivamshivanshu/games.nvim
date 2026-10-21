local board = require("games.ui.widgets.board")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")

---@alias games.PacmanSeat "player"

---@alias games.PacmanDirection "up"|"down"|"left"|"right"

---@alias games.PacmanGhostName "blinky"|"pinky"|"inky"|"clyde"

---@alias games.PacmanGhostMode "house"|"scatter"|"chase"|"frightened"|"eyes"

---@class games.PacmanActor
---@field row integer 1 at the top
---@field col integer 1 at the left
---@field facing games.PacmanDirection

---@class games.PacmanPacman: games.PacmanActor
---@field next_turn? games.PacmanDirection the turn it takes at the first cell where it can

---@class games.PacmanGhost: games.PacmanActor
---@field name games.PacmanGhostName
---@field mode games.PacmanGhostMode

---@class games.PacmanView
---@field maze string[] one line per row, top first, a tile letter per cell: # wall, - door, . dot, o pellet, space open
---@field pacman games.PacmanPacman
---@field ghosts games.PacmanGhost[]
---@field score integer
---@field lives integer counting the one in play
---@field level integer
---@field levels integer clearing the last wins
---@field food_left integer
---@field fright_ticks integer ticks the ghosts stay frightened
---@field paused boolean
---@field can_turn boolean
---@field can_pause boolean

---@class games.PacmanUpdate: games.GameUpdate
---@field view games.PacmanView

---@class games.PacmanGhostEatenEvent: games.Event
---@field kind "ghost_eaten"
---@field by games.PacmanSeat
---@field ghost games.PacmanGhostName
---@field points integer

---@class games.PacmanCaughtEvent: games.Event
---@field kind "caught"
---@field by games.PacmanSeat
---@field ghost games.PacmanGhostName
---@field lives_left integer

---@class games.PacmanLevelClearedEvent: games.Event
---@field kind "level_cleared"
---@field by games.PacmanSeat
---@field level integer

local CELL_WIDTH = 2
local TURN_KIND, PAUSE_KIND = "turn", "pause"
local KEY_HL, VALUE_HL = "GamesKey", "GamesLabel"
local PACMAN_HL, WALL_HL, DOOR_HL = "GamesPacman", "GamesPacmanWall", "GamesMuted"
local FRIGHTENED_HL, EYES_HL = "GamesGhostFrightened", "GamesMuted"
local PAUSED_TEXT, RUNNING_TEXT, OVER_TEXT = "paused: turn to go", "running", "over"
---Display columns the status panel reserves: the longest label, a space, and the widest value.
local PANEL_WIDTH = #"lives" + 1 + #PAUSED_TEXT

---@type table<string, string> asset key of each tile letter of View.maze
local TILE_KEYS =
  { ["#"] = "wall", ["-"] = "door", ["."] = "dot", ["o"] = "pellet", [" "] = "empty" }
---@type table<string, games.Hl|nil>
local TILE_HLS = { wall = WALL_HL, door = DOOR_HL }
---@type games.PacmanGhostName[]
local GHOST_NAMES = { "blinky", "pinky", "inky", "clyde" }
---@type table<games.PacmanGhostName, games.Hl>
local GHOST_HLS = {
  blinky = "GamesGhostBlinky",
  pinky = "GamesGhostPinky",
  inky = "GamesGhostInky",
  clyde = "GamesGhostClyde",
}
---@type games.PacmanDirection[]
local DIRECTIONS = { "up", "down", "left", "right" }

---@class (private) games.PacmanDirectionKey
---@field key string
---@field arrow string bound too, but left out of the footer
---@field direction games.PacmanDirection

---@type games.PacmanDirectionKey[]
local DIRECTION_KEYS = {
  { key = "h", arrow = "<Left>", direction = "left" },
  { key = "j", arrow = "<Down>", direction = "down" },
  { key = "k", arrow = "<Up>", direction = "up" },
  { key = "l", arrow = "<Right>", direction = "right" },
}

---@param direction games.PacmanDirection
---@return string
local function pacman_key(direction)
  return "pacman_" .. direction
end

---@param ghost games.PacmanGhost
---@return string
local function ghost_key(ghost)
  if ghost.mode == "frightened" or ghost.mode == "eyes" then
    return ("%s_%s"):format(ghost.name, ghost.mode)
  end
  return ghost.name
end

---@param ghost games.PacmanGhost
---@return games.Hl
local function ghost_hl(ghost)
  if ghost.mode == "frightened" then
    return FRIGHTENED_HL
  end
  return ghost.mode == "eyes" and EYES_HL or GHOST_HLS[ghost.name]
end

---Who stands on each cell: Pac-Man over any ghost, an earlier ghost over a later one.
---@param view games.PacmanView
---@return table<integer, table<integer, games.GridCell>> row -> col -> cell
local function occupants(view)
  local by_cell = {}
  local function put(actor, cell)
    by_cell[actor.row] = by_cell[actor.row] or {}
    by_cell[actor.row][actor.col] = cell
  end
  for i = #view.ghosts, 1, -1 do
    local ghost = view.ghosts[i]
    put(ghost, { ghost_key(ghost), ghost_hl(ghost) })
  end
  put(view.pacman, { pacman_key(view.pacman.facing), PACMAN_HL })
  return by_cell
end

---@param view games.PacmanView
---@return string
local function state_text(view)
  if not view.can_turn then
    return OVER_TEXT
  end
  return view.paused and PAUSED_TEXT or RUNNING_TEXT
end

---@param entry games.PacmanDirectionKey
---@return games.GameAction
local function turn_action(entry)
  return {
    key = entry.key,
    aliases = { entry.arrow },
    label = entry.direction,
    action = { kind = TURN_KIND, direction = entry.direction },
    available = function(update)
      return update.view.can_turn
    end,
  }
end

local M = {}

---@type string[] keys every pacman asset pack draws
M.asset_keys = vim
  .iter({
    vim.tbl_values(TILE_KEYS),
    vim.tbl_map(pacman_key, DIRECTIONS),
    GHOST_NAMES,
    vim.tbl_map(function(name)
      return name .. "_frightened"
    end, GHOST_NAMES),
    vim.tbl_map(function(name)
      return name .. "_eyes"
    end, GHOST_NAMES),
  })
  :flatten()
  :totable()

M.glyph_width = CELL_WIDTH

---@param update games.PacmanUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local on_cell = occupants(view)
  return board({
    grid = {
      rows = #view.maze,
      cols = #view.maze[1],
      shade = "plain",
      cell_width = CELL_WIDTH,
      gap = 0,
      cell = function(r, c)
        local occupant = on_cell[r] and on_cell[r][c]
        if occupant then
          return occupant
        end
        local key = TILE_KEYS[view.maze[r]:sub(c, c)]
        return { key, TILE_HLS[key] }
      end,
    },
    art = assert(ctx.art, "pacman draws from an asset pack"),
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv({
            pairs = {
              { "score", tostring(view.score) },
              { "lives", tostring(view.lives) },
              { "level", ("%d/%d"):format(view.level, view.levels) },
              { "food", tostring(view.food_left) },
              { "game", state_text(view) },
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
M.actions = vim.list_extend(vim.tbl_map(turn_action, DIRECTION_KEYS), {
  {
    key = "p",
    label = "pause",
    action = { kind = PAUSE_KIND },
    available = function(update)
      return update.view.can_pause
    end,
  },
})

---@param event games.Event
---@param update games.PacmanUpdate
---@return string|nil
function M.describe_event(event, update)
  local who = players.user_of_seat(update, event.by) or event.by
  if event.kind == "ghost_eaten" then
    ---@cast event games.PacmanGhostEatenEvent
    return ("%s ate %s +%d"):format(who, event.ghost, event.points)
  end
  if event.kind == "caught" then
    ---@cast event games.PacmanCaughtEvent
    local lives = event.lives_left == 1 and "1 life" or ("%d lives"):format(event.lives_left)
    return ("%s caught %s, %s left"):format(event.ghost, who, lives)
  end
  if event.kind == "level_cleared" then
    ---@cast event games.PacmanLevelClearedEvent
    return ("%s cleared level %d"):format(who, event.level)
  end
  return nil
end

return M
