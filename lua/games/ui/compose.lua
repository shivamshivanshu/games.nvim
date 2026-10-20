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

---@alias games.Node games.TextNode|games.RowNode|games.ColNode|games.PadNode

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

---@param extmarks games.Extmark[]
---@param row_offset integer
---@param col_offset integer
---@return games.Extmark[]
local function shifted(extmarks, row_offset, col_offset)
  return vim.tbl_map(function(mark)
    return {
      row = mark.row + row_offset,
      col = mark.col + col_offset,
      end_col = mark.end_col + col_offset,
      hl = mark.hl,
    }
  end, extmarks)
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
    local width = vim.fn.strdisplaywidth(str)
    return { lines = { str }, extmarks = extmarks, widths = { width }, width = width }
  end

  if node.kind == "pad" then
    local child = layout(node.child)
    local indent = (" "):rep(node.n)
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
    local marks_by_row = vim.defaulttable()
    for _, mark in ipairs(block.extmarks) do
      table.insert(marks_by_row[mark.row], mark)
    end
    for i = 1, height do
      local line_prefix_bytes = #lines[i]
      vim.list_extend(extmarks, shifted(marks_by_row[i - 1], 0, line_prefix_bytes))
      local shortfall = block.width - (block.widths[i] or 0)
      lines[i] = lines[i] .. (block.lines[i] or "") .. (" "):rep(shortfall)
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

return M
