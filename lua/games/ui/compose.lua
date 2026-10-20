---@class games.Extmark
---@field row integer 0-based line index
---@field col integer byte column
---@field end_col integer byte column, exclusive
---@field hl string

---@class games.Rendered
---@field lines string[]
---@field extmarks games.Extmark[]

---@class games.Node
---@field kind "text"|"row"|"col"|"pad"
---@field str? string
---@field hl? string
---@field children? games.Node[]
---@field n? integer
---@field child? games.Node

---@class games.Block
---@field lines string[]
---@field extmarks games.Extmark[]
---@field widths integer[] display width of each line
---@field width integer display width of the widest line

local M = {}

---@param str string
---@param hl? string
---@return games.Node
function M.text(str, hl)
  return { kind = "text", str = str, hl = hl }
end

---@param children games.Node[]
---@return games.Node
function M.row(children)
  return { kind = "row", children = children }
end

---@param children games.Node[]
---@return games.Node
function M.col(children)
  return { kind = "col", children = children }
end

---@param n integer blank columns added on the left
---@param node games.Node
---@return games.Node
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
