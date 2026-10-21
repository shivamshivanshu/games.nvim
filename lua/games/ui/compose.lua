local text_util = require("games.ui.text")

---@alias games.Hl string|string[] one group, or groups layered so later ones override only the attributes they set

---@class games.Extmark
---@field row integer 0-based line index
---@field col integer byte column
---@field end_col integer byte column, exclusive
---@field hl games.Hl

---@class games.Rendered
---@field lines string[]
---@field extmarks games.Extmark[]

---@class games.TextNode
---@field kind "text"
---@field str string
---@field hl? games.Hl

---@class games.RowNode
---@field kind "row"
---@field children games.Node[]

---@class games.ColNode
---@field kind "col"
---@field children games.Node[]

---@class games.PadNode
---@field kind "pad"
---@field n integer blank columns added on the left
---@field child games.Node

---@class games.BoxNode
---@field kind "box"
---@field w integer display columns
---@field h integer lines
---@field hl? games.Hl over every whole line of the box, under the child's own highlights
---@field child games.Node centred; odd padding goes right and bottom

---@alias games.Node games.TextNode|games.RowNode|games.ColNode|games.PadNode|games.BoxNode

---@class games.Size
---@field w integer display columns
---@field h integer lines

---Lines being laid out, each node written once at its absolute row and column.
---@class (private) games.Canvas
---@field fragments table<integer, string[]> by 0-based row, joined into its line at the end
---@field bytes table<integer, integer> by 0-based row, bytes written so far
---@field cols table<integer, integer> by 0-based row, the display column the nodes written so far report reaching
---@field extmarks games.Extmark[]

local M = {}

---@param str string
---@param hl? games.Hl
---@return games.TextNode
function M.text(str, hl)
  return { kind = "text", str = str, hl = hl }
end

---Layers that are set, in order, flattened into one highlight; nil when none is set.
---@param ... games.Hl|nil
---@return games.Hl|nil
function M.layers(...)
  local groups = {}
  for i = 1, select("#", ...) do
    local hl = select(i, ...)
    if type(hl) == "table" then
      vim.list_extend(groups, hl)
    elseif hl then
      groups[#groups + 1] = hl
    end
  end
  if #groups <= 1 then
    return groups[1]
  end
  return groups
end

---@param children games.Node[]
---@return games.RowNode
function M.row(children)
  return { kind = "row", children = children }
end

---@param children games.Node[]
---@return games.ColNode
function M.col(children)
  return { kind = "col", children = children }
end

---@param n integer blank columns added on the left
---@param node games.Node
---@return games.PadNode
function M.pad(n, node)
  return { kind = "pad", n = n, child = node }
end

---@class games.BoxProps
---@field w integer display columns
---@field h integer lines
---@field hl? games.Hl over every whole line of the box
---@field child games.Node centred; odd padding goes right and bottom

---A w x h block filled with hl on every line, with child centred in it.
---@param props games.BoxProps
---@return games.BoxNode
function M.box(props)
  return { kind = "box", w = props.w, h = props.h, hl = props.hl, child = props.child }
end

---Pads row with spaces out to display column col, or takes col as reached when it is past it.
---@param canvas games.Canvas
---@param row integer
---@param col integer
local function advance(canvas, row, col)
  local reached = canvas.cols[row]
  if not reached then
    canvas.fragments[row], canvas.bytes[row], reached = {}, 0, 0
  end
  if col > reached then
    local fragments = canvas.fragments[row]
    fragments[#fragments + 1] = text_util.spaces(col - reached)
    canvas.bytes[row] = canvas.bytes[row] + col - reached
  end
  canvas.cols[row] = col
end

local place

---@param canvas games.Canvas
---@param node games.TextNode
---@param row integer
---@param col integer
---@return integer width
---@return integer height
local function place_text(canvas, node, row, col)
  local str = node.str or ""
  advance(canvas, row, col)
  if #str > 0 then
    local start = canvas.bytes[row]
    if node.hl then
      canvas.extmarks[#canvas.extmarks + 1] =
        { row = row, col = start, end_col = start + #str, hl = node.hl }
    end
    local fragments = canvas.fragments[row]
    fragments[#fragments + 1] = str
    canvas.bytes[row] = start + #str
  end
  local width = text_util.width(str)
  canvas.cols[row] = col + width
  return width, 1
end

---@param canvas games.Canvas
---@param node games.BoxNode
---@param row integer
---@param col integer
---@return integer width
---@return integer height
local function place_box(canvas, node, row, col)
  local child = M.measure(node.child)
  local line_marks = {}
  for i = 0, node.h - 1 do
    advance(canvas, row + i, col)
    if node.hl then
      local mark = { row = row + i, col = canvas.bytes[row + i], end_col = 0, hl = node.hl }
      line_marks[i] = mark
      canvas.extmarks[#canvas.extmarks + 1] = mark
    end
  end
  local top = math.max(0, math.floor((node.h - child.h) / 2))
  local left = math.max(0, math.floor((node.w - child.w) / 2))
  place(canvas, node.child, row + top, col + left)
  for i = 0, node.h - 1 do
    advance(canvas, row + i, col + node.w)
    if line_marks[i] then
      line_marks[i].end_col = canvas.bytes[row + i]
    end
  end
  return node.w, node.h
end

---@param canvas games.Canvas
---@param node games.RowNode
---@param row integer
---@param col integer
---@return integer width
---@return integer height
local function place_row(canvas, node, row, col)
  local width, height = 0, 0
  for _, child in ipairs(node.children) do
    local w, h = place(canvas, child, row, col + width)
    width, height = width + w, math.max(height, h)
  end
  for i = 0, height - 1 do
    advance(canvas, row + i, col + width)
  end
  return width, height
end

---@param canvas games.Canvas
---@param node games.ColNode
---@param row integer
---@param col integer
---@return integer width
---@return integer height
local function place_col(canvas, node, row, col)
  local width, height = 0, 0
  for _, child in ipairs(node.children) do
    local w, h = place(canvas, child, row + height, col)
    width, height = math.max(width, w), height + h
  end
  return width, height
end

---Writes node with its top left at row and col; a line's columns left of col are filled first.
---@param canvas games.Canvas
---@param node games.Node
---@param row integer
---@param col integer
---@return integer width
---@return integer height
place = function(canvas, node, row, col)
  if node.kind == "text" then
    return place_text(canvas, node, row, col)
  end
  if node.kind == "pad" then
    local w, h = place(canvas, node.child, row, col + node.n)
    return w + node.n, h
  end
  if node.kind == "box" then
    return place_box(canvas, node, row, col)
  end
  if node.kind == "col" then
    return place_col(canvas, node, row, col)
  end
  return place_row(canvas, node, row, col)
end

---@param node games.Node
---@return games.Rendered
function M.render(node)
  local canvas = { fragments = {}, bytes = {}, cols = {}, extmarks = {} }
  local _, height = place(canvas, node, 0, 0)
  local lines = {}
  for row = 0, height - 1 do
    lines[row + 1] = table.concat(canvas.fragments[row])
  end
  return { lines = lines, extmarks = canvas.extmarks }
end

---The size render would give node, without laying it out.
---@param node games.Node
---@return games.Size
function M.measure(node)
  if node.kind == "text" then
    return { w = text_util.width(node.str or ""), h = 1 }
  end
  if node.kind == "pad" then
    local child = M.measure(node.child)
    return { w = child.w + node.n, h = child.h }
  end
  if node.kind == "box" then
    return { w = node.w, h = node.h }
  end
  local w, h = 0, 0
  if node.kind == "col" then
    for _, child in ipairs(node.children) do
      local size = M.measure(child)
      w, h = math.max(w, size.w), h + size.h
    end
  else
    for _, child in ipairs(node.children) do
      local size = M.measure(child)
      w, h = w + size.w, math.max(h, size.h)
    end
  end
  return { w = w, h = h }
end

---@param size games.Size
---@param space games.Size
---@return boolean
local function fits(size, space)
  return size.w <= space.w and size.h <= space.h
end

---The first candidate that fits space, and its size, building only the candidates a binary
---search probes. Candidates must be monotone: once one fits, every later one fits too.
---@param space games.Size
---@param candidates (fun(): games.Node)[] largest first
---@return games.Node|nil
---@return games.Size|nil
function M.first_fit(space, candidates)
  local built, sizes = {}, {}
  local function fitting(i)
    built[i] = built[i] or candidates[i]()
    sizes[i] = sizes[i] or M.measure(built[i])
    return fits(sizes[i], space)
  end
  local low, high = 1, #candidates + 1
  while low < high do
    local mid = math.floor((low + high) / 2)
    if fitting(mid) then
      high = mid
    else
      low = mid + 1
    end
  end
  return built[low], sizes[low]
end

return M
