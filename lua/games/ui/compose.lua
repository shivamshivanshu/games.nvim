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

---@param lines string[]
---@return integer
local function max_display_width(lines)
  local width = 0
  for _, line in ipairs(lines) do
    width = math.max(width, vim.fn.strdisplaywidth(line))
  end
  return width
end

---@param line string
---@param width integer
---@return string
local function pad_to_width(line, width)
  return line .. (" "):rep(width - vim.fn.strdisplaywidth(line))
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
    return { lines = { str }, extmarks = extmarks, width = vim.fn.strdisplaywidth(str) }
  end

  if node.kind == "pad" then
    local child = layout(node.child)
    local indent = (" "):rep(node.n)
    local lines = vim.tbl_map(function(line)
      return indent .. line
    end, child.lines)
    return {
      lines = lines,
      extmarks = shifted(child.extmarks, 0, node.n),
      width = child.width + node.n,
    }
  end

  local blocks = vim.tbl_map(layout, node.children)

  if node.kind == "col" then
    local lines, extmarks = {}, {}
    for _, block in ipairs(blocks) do
      vim.list_extend(extmarks, shifted(block.extmarks, #lines, 0))
      vim.list_extend(lines, block.lines)
    end
    return { lines = lines, extmarks = extmarks, width = max_display_width(lines) }
  end

  local height = 0
  for _, block in ipairs(blocks) do
    height = math.max(height, #block.lines)
  end
  local lines, extmarks = {}, {}
  for i = 1, height do
    lines[i] = ""
  end
  for _, block in ipairs(blocks) do
    for i = 1, height do
      local line_prefix_bytes = #lines[i]
      local cell = pad_to_width(block.lines[i] or "", block.width)
      for _, mark in ipairs(block.extmarks) do
        if mark.row == i - 1 then
          local moved = shifted({ mark }, 0, line_prefix_bytes)
          extmarks[#extmarks + 1] = moved[1]
        end
      end
      lines[i] = lines[i] .. cell
    end
  end
  return { lines = lines, extmarks = extmarks, width = max_display_width(lines) }
end

---@param node games.Node
---@return games.Rendered
function M.render(node)
  local block = layout(node)
  return { lines = block.lines, extmarks = block.extmarks }
end

return M
