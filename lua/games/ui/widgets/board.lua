local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")

local PANEL_GAP = 3
---Display columns per cell line: terminal cells are about twice as tall as wide.
local CELL_ASPECT = 2

---@class games.BoardPanel
---@field node fun(max_height: integer): games.Node drawn beside the board, at most max_height lines
---@field width? integer display columns it reserves; required when its width follows game state

---@class games.BoardProps
---@field grid games.GridProps the board sets cell_height, art, and cell_width above height 1
---@field art? games.ArtSet
---@field panels? games.BoardPanel[] beside the board, left to right

---@param props games.BoardProps
---@param cell_height integer
---@return games.Node
local function grid_at(props, cell_height)
  local scaled = { cell_height = cell_height, art = props.art }
  if cell_height > 1 then
    scaled.cell_width = CELL_ASPECT * cell_height
  end
  return grid(vim.tbl_extend("force", props.grid, scaled))
end

---@param panels games.BoardPanel[]
---@param space games.Size
---@return integer display columns the panels and their gaps take
local function panels_width(panels, space)
  return vim.iter(panels):fold(0, function(width, panel)
    return width + PANEL_GAP + (panel.width or compose.measure(panel.node(space.h)).w)
  end)
end

---The tallest cell that space and ctx.max_cell_height allow, at least 1.
---@param props games.BoardProps
---@param space games.Size|nil
---@param max_cell_height integer|nil
---@return integer
local function tallest_cell(props, space, max_cell_height)
  if not space then
    return 1
  end
  local tallest = math.floor(space.h / props.grid.rows)
  return math.max(1, math.min(tallest, max_cell_height or tallest))
end

---A grid scaled to fill ctx.space, its cells as tall as fit, with panels beside it. Without a
---space the cells are one line tall.
---@param props games.BoardProps
---@param ctx games.LayoutCtx
---@return games.Node
return function(props, ctx)
  local panels = props.panels or {}
  local space = ctx.space
  local candidates = {}
  for cell_height = tallest_cell(props, space, ctx.max_cell_height), 1, -1 do
    candidates[#candidates + 1] = function()
      return grid_at(props, cell_height)
    end
  end
  local fitted = space
    and compose.first_fit({ w = space.w - panels_width(panels, space), h = space.h }, candidates)
  local board = fitted or candidates[#candidates]()
  local max_height = compose.measure(board).h
  local parts = { board }
  for _, panel in ipairs(panels) do
    parts[#parts + 1] = compose.pad(PANEL_GAP, panel.node(max_height))
  end
  return compose.row(parts)
end
