local actions = require("games.ui.actions")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local keys = require("games.ui.keys")
local kv = require("games.ui.widgets.kv")

local Direction = keys.Direction
local Group = highlights.Group

---@alias games.PacmanSeat "player"

---@alias games.PacmanGhostName "blinky"|"pinky"|"inky"|"clyde"

---@alias games.PacmanGhostMode "house"|"scatter"|"chase"|"frightened"|"eyes"

---@class games.PacmanActor
---@field row integer 1 at the top
---@field col integer 1 at the left
---@field facing games.Direction

---@class games.PacmanPacman: games.PacmanActor
---@field next_turn? games.Direction the turn it takes at the first cell where it can

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
local WALL_HL, DOOR_HL = Group.PACMAN_WALL, Group.MUTED
local FRIGHTENED_HL, EYES_HL = Group.GHOST_FRIGHTENED, Group.MUTED
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
  blinky = Group.GHOST_BLINKY,
  pinky = Group.GHOST_PINKY,
  inky = Group.GHOST_INKY,
  clyde = Group.GHOST_CLYDE,
}
---@type games.Direction[]
local DIRECTIONS = { Direction.UP, Direction.DOWN, Direction.LEFT, Direction.RIGHT }

---@param direction games.Direction
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
---@return table<integer, table<integer, games.PacmanActor>> row -> col -> actor
local function occupants(view)
  local actors = vim.iter(view.ghosts):rev():totable()
  actors[#actors + 1] = view.pacman
  return grid.cell_index(actors)
end

---@param view games.PacmanView
---@param actor games.PacmanActor
---@return games.GridCell
local function actor_cell(view, actor)
  if actor == view.pacman then
    return { pacman_key(view.pacman.facing), Group.PACMAN }
  end
  ---@cast actor games.PacmanGhost
  return { ghost_key(actor), ghost_hl(actor) }
end

---@param update games.PacmanUpdate
---@return string
local function state_text(update)
  if update.result then
    return OVER_TEXT
  end
  return update.view.paused and PAUSED_TEXT or RUNNING_TEXT
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
      cell_width = CELL_WIDTH,
      gap = 0,
      cell = function(r, c)
        local actor = on_cell[r] and on_cell[r][c]
        if actor then
          return actor_cell(view, actor)
        end
        local key = TILE_KEYS[view.maze[r]:sub(c, c)]
        return { key, TILE_HLS[key] }
      end,
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv.panel({
            { "score", tostring(view.score) },
            { "lives", tostring(view.lives) },
            { "level", ("%d/%d"):format(view.level, view.levels) },
            { "food", tostring(view.food_left) },
            { "game", state_text(update) },
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.list_extend(
  actions.directions({
    kind = TURN_KIND,
    arrows = true,
    movable = function(view)
      return view.can_turn
    end,
  }),
  {
    {
      key = "p",
      label = "pause",
      action = { kind = PAUSE_KIND },
      available = function(update)
        return update.view.can_pause
      end,
    },
  }
)

---@type table<games.EventKind, games.Describer>
M.describers = {
  ghost_eaten = function(event, talk)
    ---@cast event games.PacmanGhostEatenEvent
    return ("%s ate %s +%d"):format(talk.name(event.by), event.ghost, event.points)
  end,
  caught = function(event, talk)
    ---@cast event games.PacmanCaughtEvent
    local lives = event.lives_left == 1 and "1 life" or ("%d lives"):format(event.lives_left)
    return ("%s caught %s, %s left"):format(event.ghost, talk.name(event.by), lives)
  end,
  level_cleared = function(event, talk)
    ---@cast event games.PacmanLevelClearedEvent
    return ("%s cleared level %d"):format(talk.name(event.by), event.level)
  end,
}

return M
