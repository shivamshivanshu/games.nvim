local actions = require("games.ui.actions")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local keys = require("games.ui.keys")
local kv = require("games.ui.widgets.kv")

local Direction = keys.Direction
local Group = highlights.Group

---@alias games.PacmanSeat "solver"

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
---@type table<string, games.GridCell> cell of each tile letter of View.maze
local TILE_CELLS = vim.iter(TILE_KEYS):fold({}, function(cells, letter, key)
  cells[letter] = { key, TILE_HLS[key] }
  return cells
end)
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

---@param name games.PacmanGhostName
---@param mode games.PacmanGhostMode
---@return string
local function ghost_key(name, mode)
  if mode == "frightened" or mode == "eyes" then
    return ("%s_%s"):format(name, mode)
  end
  return name
end

---@param name games.PacmanGhostName
---@param mode games.PacmanGhostMode
---@return games.Hl
local function ghost_hl(name, mode)
  if mode == "frightened" then
    return FRIGHTENED_HL
  end
  return mode == "eyes" and EYES_HL or GHOST_HLS[name]
end

---@param name games.PacmanGhostName
---@param mode games.PacmanGhostMode
---@return games.GridCell
local function ghost_cell(name, mode)
  return { ghost_key(name, mode), ghost_hl(name, mode) }
end

---Pac-Man and the ghosts, in the order they are drawn over each other: the last on a cell shows.
---@param view games.PacmanView
---@return games.PacmanActor[]
local function actors_of(view)
  local actors = vim.iter(view.ghosts):rev():totable()
  actors[#actors + 1] = view.pacman
  return actors
end

---@param view games.PacmanView
---@param actor games.PacmanActor
---@return games.GridCell
local function actor_cell(view, actor)
  if actor == view.pacman then
    return { pacman_key(view.pacman.facing), Group.PACMAN }
  end
  ---@cast actor games.PacmanGhost
  return ghost_cell(actor.name, actor.mode)
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
  local actors = actors_of(view)
  local on_cell = grid.cell_index(actors)
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
        return TILE_CELLS[view.maze[r]:sub(c, c)]
      end,
      row_key = function(r)
        local key = view.maze[r]
        for _, actor in ipairs(actors) do
          if actor.row == r then
            local cell = actor_cell(view, actor)
            key = ("%s|%d %s %s"):format(key, actor.col, cell[1], cell[2])
          end
        end
        return key
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

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function tile(key, meaning)
    return grid.legend_entry(ctx, { key, TILE_HLS[key] }, meaning)
  end
  local ghosts = grid.samples(
    ctx,
    vim.tbl_map(function(name)
      return ghost_cell(name, "chase")
    end, GHOST_NAMES)
  )
  local first = GHOST_NAMES[1]
  return {
    grid.legend_entry(
      ctx,
      { pacman_key(Direction.RIGHT), Group.PACMAN },
      "Pac-Man, facing the way it moves"
    ),
    { sample = ghosts, meaning = "the ghosts: blinky, pinky, inky and clyde" },
    grid.legend_entry(ctx, ghost_cell(first, "frightened"), "a frightened ghost"),
    grid.legend_entry(ctx, ghost_cell(first, "eyes"), "a ghost's eyes, going home"),
    tile("dot", "a dot"),
    tile("pellet", "a pellet, which frightens the ghosts"),
    tile("wall", "a wall"),
    tile("door", "the door of the ghost house"),
  }
end

return M
