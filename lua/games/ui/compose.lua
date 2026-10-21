local text_util = require("games.ui.text")

---@alias games.Hl string|string[] one group, or groups layered so later ones override only the attributes they set

---Extmarks as parallel arrays: mark i spans row[i], from col[i] to end_col[i], in hl[i].
---@class games.Marks
---@field row integer[] 0-based line index
---@field col integer[] byte column
---@field end_col integer[] byte column, exclusive
---@field hl games.Hl[]

---@class games.Rendered
---@field lines string[]
---@field marks games.Marks

---@class games.TextNode
---@field kind "text"
---@field str string
---@field hl? games.Hl
---@field w? integer display columns of str, memoised by the first layout that measures it

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

---Lines drawn already, each w display columns wide, placed as they are.
---@class games.BlockNode
---@field kind "block"
---@field w integer display columns of every line
---@field h integer lines
---@field lines string[]
---@field marks games.Marks rows count from the block's first line, byte columns from its left edge

---@alias games.Node games.TextNode|games.RowNode|games.ColNode|games.PadNode|games.BlockNode

---@class games.Size
---@field w integer display columns
---@field h integer lines

---Lines being laid out, each node written once at its absolute row and column.
---@class (private) games.Canvas
---@field fragments table<integer, string[]> by 0-based row, joined into its line at the end
---@field bytes table<integer, integer> by 0-based row, bytes written so far
---@field cols table<integer, integer> by 0-based row, the display column the nodes written so far report reaching
---@field marks games.Marks

local M = {}

---@return games.Marks
function M.no_marks()
  return { row = {}, col = {}, end_col = {}, hl = {} }
end

---Appends a mark to marks.
---@param marks games.Marks
---@param row integer
---@param col integer
---@param end_col integer
---@param hl games.Hl
---@return integer index of the mark
function M.add_mark(marks, row, col, end_col, hl)
  local i = #marks.row + 1
  marks.row[i], marks.col[i], marks.end_col[i], marks.hl[i] = row, col, end_col, hl
  return i
end

---Whether a and b layer the same groups.
---@param a games.Hl|nil
---@param b games.Hl|nil
---@return boolean
function M.same_hl(a, b)
  if type(a) ~= "table" or type(b) ~= "table" then
    return a == b
  end
  if #a ~= #b then
    return false
  end
  for i = 1, #a do
    if a[i] ~= b[i] then
      return false
    end
  end
  return true
end

---@param str string
---@param hl? games.Hl
---@return games.TextNode
function M.text(str, hl)
  return { kind = "text", str = str, hl = hl }
end

---@param w integer display columns of every line
---@param lines string[]
---@param marks games.Marks rows count from the first line, byte columns from the left edge
---@return games.BlockNode
function M.block(w, lines, marks)
  return { kind = "block", w = w, h = #lines, lines = lines, marks = marks }
end

---Layers that are set, in order, flattened into one highlight; nil when none is set.
---@param ... games.Hl|nil
---@return games.Hl|nil
function M.layers(...)
  local set, only = 0, nil
  for i = 1, select("#", ...) do
    local hl = select(i, ...)
    if hl then
      set, only = set + 1, hl
    end
  end
  if set == 0 or (set == 1 and type(only) == "string") then
    return only
  end
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

---@param node games.TextNode
---@return integer
local function text_width(node)
  local width = node.w
  if not width then
    width = text_util.width(node.str or "")
    node.w = width
  end
  return width
end

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
      M.add_mark(canvas.marks, row, start, start + #str, node.hl)
    end
    local fragments = canvas.fragments[row]
    fragments[#fragments + 1] = str
    canvas.bytes[row] = start + #str
  end
  local width = text_width(node)
  canvas.cols[row] = col + width
  return width, 1
end

---@param canvas games.Canvas
---@param node games.BlockNode
---@param row integer
---@param col integer
---@return integer width
---@return integer height
local function place_block(canvas, node, row, col)
  local starts = {}
  for i, line in ipairs(node.lines) do
    local at = row + i - 1
    advance(canvas, at, col)
    local fragments = canvas.fragments[at]
    fragments[#fragments + 1] = line
    starts[i] = canvas.bytes[at]
    canvas.bytes[at] = starts[i] + #line
    canvas.cols[at] = col + node.w
  end
  local marks = node.marks
  for i = 1, #marks.row do
    local start = starts[marks.row[i] + 1]
    M.add_mark(
      canvas.marks,
      row + marks.row[i],
      start + marks.col[i],
      start + marks.end_col[i],
      marks.hl[i]
    )
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
  if node.kind == "block" then
    return place_block(canvas, node, row, col)
  end
  if node.kind == "col" then
    return place_col(canvas, node, row, col)
  end
  return place_row(canvas, node, row, col)
end

---@param node games.Node
---@return games.Rendered
function M.render(node)
  local canvas = { fragments = {}, bytes = {}, cols = {}, marks = M.no_marks() }
  local _, height = place(canvas, node, 0, 0)
  local lines = {}
  for row = 0, height - 1 do
    lines[row + 1] = table.concat(canvas.fragments[row])
  end
  return { lines = lines, marks = canvas.marks }
end

---The size render would give node, without laying it out.
---@param node games.Node
---@return games.Size
function M.measure(node)
  if node.kind == "text" then
    return { w = text_width(node), h = 1 }
  end
  if node.kind == "pad" then
    local child = M.measure(node.child)
    return { w = child.w + node.n, h = child.h }
  end
  if node.kind == "block" then
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
