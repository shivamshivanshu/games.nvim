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

---@class games.Block: games.Rendered
---@field widths integer[] display width of each line
---@field width integer display width of the widest line

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

---@param mark games.Extmark
---@param row_offset integer
---@param col_offset integer
---@return games.Extmark
local function shifted_mark(mark, row_offset, col_offset)
  return {
    row = mark.row + row_offset,
    col = mark.col + col_offset,
    end_col = mark.end_col + col_offset,
    hl = mark.hl,
  }
end

---@param extmarks games.Extmark[]
---@param row_offset integer
---@param col_offset integer
---@return games.Extmark[]
local function shifted(extmarks, row_offset, col_offset)
  return vim.tbl_map(function(mark)
    return shifted_mark(mark, row_offset, col_offset)
  end, extmarks)
end

---@param extmarks games.Extmark[]
---@return table<integer, games.Extmark[]> by row, holding only rows that have marks
local function marks_by_row(extmarks)
  local by_row = {}
  for _, mark in ipairs(extmarks) do
    local row_marks = by_row[mark.row]
    if not row_marks then
      row_marks = {}
      by_row[mark.row] = row_marks
    end
    row_marks[#row_marks + 1] = mark
  end
  return by_row
end

---@param node games.BoxNode
---@param child games.Block
---@return games.Block
local function layout_box(node, child)
  local top = math.max(0, math.floor((node.h - #child.lines) / 2))
  local left = math.max(0, math.floor((node.w - child.width) / 2))
  local indent = text_util.spaces(left)
  local lines, extmarks, widths = {}, {}, {}
  for i = 1, node.h do
    local child_row = i - top
    local child_line = child.lines[child_row]
    if child_line then
      lines[i] = indent .. child_line .. text_util.spaces(node.w - left - child.widths[child_row])
    else
      lines[i] = text_util.spaces(node.w)
    end
    widths[i] = node.w
    if node.hl then
      extmarks[#extmarks + 1] = { row = i - 1, col = 0, end_col = #lines[i], hl = node.hl }
    end
  end
  vim.list_extend(extmarks, shifted(child.extmarks, top, left))
  return { lines = lines, extmarks = extmarks, widths = widths, width = node.w }
end

---@param node games.Node
---@return games.Block
local function layout(node)
  if node.kind == "text" then
    local str = node.str or ""
    local extmarks = {}
    if node.hl and #str > 0 then
      extmarks[1] = { row = 0, col = 0, end_col = #str, hl = node.hl }
    end
    local width = text_util.width(str)
    return { lines = { str }, extmarks = extmarks, widths = { width }, width = width }
  end

  if node.kind == "pad" then
    local child = layout(node.child)
    local indent = text_util.spaces(node.n)
    return {
      lines = vim.tbl_map(function(line)
        return indent .. line
      end, child.lines),
      extmarks = shifted(child.extmarks, 0, node.n),
      widths = vim.tbl_map(function(width)
        return width + node.n
      end, child.widths),
      width = child.width + node.n,
    }
  end

  if node.kind == "box" then
    return layout_box(node, layout(node.child))
  end

  local blocks = vim.tbl_map(layout, node.children)

  if node.kind == "col" then
    local lines, extmarks, widths, width = {}, {}, {}, 0
    for _, block in ipairs(blocks) do
      vim.list_extend(extmarks, shifted(block.extmarks, #lines, 0))
      vim.list_extend(lines, block.lines)
      vim.list_extend(widths, block.widths)
      width = math.max(width, block.width)
    end
    return { lines = lines, extmarks = extmarks, widths = widths, width = width }
  end

  local height, width = 0, 0
  for _, block in ipairs(blocks) do
    height = math.max(height, #block.lines)
    width = width + block.width
  end
  local lines, extmarks, widths = {}, {}, {}
  for i = 1, height do
    lines[i] = ""
    widths[i] = width
  end
  for _, block in ipairs(blocks) do
    local block_marks = marks_by_row(block.extmarks)
    for i = 1, height do
      local line_prefix_bytes = #lines[i]
      local row_marks = block_marks[i - 1]
      if row_marks then
        for _, mark in ipairs(row_marks) do
          extmarks[#extmarks + 1] = shifted_mark(mark, 0, line_prefix_bytes)
        end
      end
      local shortfall = block.width - (block.widths[i] or 0)
      lines[i] = lines[i] .. (block.lines[i] or "") .. text_util.spaces(shortfall)
    end
  end
  return { lines = lines, extmarks = extmarks, widths = widths, width = width }
end

---@param node games.Node
---@return games.Rendered
function M.render(node)
  local block = layout(node)
  return { lines = block.lines, extmarks = block.extmarks }
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
