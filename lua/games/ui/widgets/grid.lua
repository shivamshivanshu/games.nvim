local compose = require("games.ui.compose")
local text_util = require("games.ui.text")

---@class games.GridCell
---@field [1] string glyph
---@field [2]? string highlight group overriding the shade

---@class games.GridLabels
---@field cols string one character per column
---@field rows string one character per row

---@class games.GridProps
---@field rows integer
---@field cols integer
---@field cell fun(row: integer, col: integer): games.GridCell
---@field shade "checker"|"plain"
---@field labels? games.GridLabels
---@field marks? table<string, string> mark_key(row, col) -> highlight group
---@field flip? boolean
---@field cell_width? integer display columns per cell, default 1
---@field blocks? integer positive: draw a thin separator after every this many rows and columns
---@field glyphs? games.Glyphs separator characters, default "unicode"

local SHADE_BY_PARITY = { [0] = "GamesSquareLight", [1] = "GamesSquareDark" }
local CELL_GAP = " "

---@class (private) games.BlockSeparators
---@field col string
---@field row string
---@field crossing string

---@type table<games.Glyphs, games.BlockSeparators>
local BLOCK_SEPARATORS = {
  unicode = { col = "│", row = "─", crossing = "┼" },
  ascii = { col = "|", row = "-", crossing = "+" },
}
local SEPARATOR_HL = "GamesMuted"

---@class games.GridWidget
---@field mark_key fun(row: integer, col: integer): string
---@overload fun(props: games.GridProps): games.Node

local M = {}

---Key of the cell at logical row and col in GridProps.marks.
---@param row integer
---@param col integer
---@return string
function M.mark_key(row, col)
  return row .. "," .. col
end

---@param n integer
---@param flip boolean
---@return integer[]
local function display_order(n, flip)
  local order = {}
  for i = 1, n do
    order[i] = flip and n + 1 - i or i
  end
  return order
end

---@param labels string
---@param order integer[]
---@return string[]
local function labels_in_order(labels, order)
  local chars = vim.fn.split(labels, "\\zs")
  return vim.tbl_map(function(i)
    return chars[i]
  end, order)
end

---Whether a block boundary lies between the adjacent logical indices a and b.
---@param blocks integer|nil
---@param a integer
---@param b integer
---@return boolean
local function crosses_block(blocks, a, b)
  return blocks ~= nil and math.min(a, b) % blocks == 0
end

---@param props games.GridProps
---@param col_order integer[]
---@param prefix_width integer display width of the row label column
---@param separators games.BlockSeparators
---@return games.Node
local function block_separator_row(props, col_order, prefix_width, separators)
  local cell = separators.row:rep(props.cell_width or 1)
  local parts = { (" "):rep(prefix_width), cell }
  for j = 2, #col_order do
    local crossing = crosses_block(props.blocks, col_order[j - 1], col_order[j])
    parts[#parts + 1] = crossing and separators.crossing or separators.row
    parts[#parts + 1] = cell
  end
  return compose.text(table.concat(parts), SEPARATOR_HL)
end

---@param blocks integer|nil
local function validate_blocks(blocks)
  local valid = blocks == nil
    or (type(blocks) == "number" and blocks >= 1 and blocks == math.floor(blocks))
  if not valid then
    error(("grid: blocks must be a positive integer or nil, got %s"):format(vim.inspect(blocks)), 2)
  end
end

---@param props games.GridProps
---@return games.Node
local function render(props)
  validate_blocks(props.blocks)
  local separators = BLOCK_SEPARATORS[props.glyphs or "unicode"]
  local cell_width = props.cell_width or 1
  local marks = props.marks or {}
  local row_order = display_order(props.rows, props.flip == true)
  local col_order = display_order(props.cols, props.flip == true)
  local row_labels = props.labels and labels_in_order(props.labels.rows, row_order)
  local col_labels = props.labels and labels_in_order(props.labels.cols, col_order)
  local prefix_width = row_labels and vim.fn.strdisplaywidth(row_labels[1] .. " ") or 0

  local function cell_node(r, c)
    local glyph, own_hl = unpack(props.cell(r, c))
    local shade = props.shade == "checker" and SHADE_BY_PARITY[(r + c) % 2] or nil
    local hl = marks[M.mark_key(r, c)] or own_hl or shade
    return compose.text(text_util.pad_to_width(glyph, cell_width), hl)
  end

  local function gap_node(j)
    if crosses_block(props.blocks, col_order[j - 1], col_order[j]) then
      return compose.text(separators.col, SEPARATOR_HL)
    end
    return compose.text(CELL_GAP)
  end

  local lines = {}
  for i, r in ipairs(row_order) do
    if i > 1 and crosses_block(props.blocks, row_order[i - 1], r) then
      lines[#lines + 1] = block_separator_row(props, col_order, prefix_width, separators)
    end
    local parts = {}
    if row_labels then
      parts[#parts + 1] = compose.text(row_labels[i] .. " ")
    end
    for j, c in ipairs(col_order) do
      if j > 1 then
        parts[#parts + 1] = gap_node(j)
      end
      parts[#parts + 1] = cell_node(r, c)
    end
    lines[#lines + 1] = compose.row(parts)
  end

  if col_labels then
    local padded = vim.tbl_map(function(label)
      return text_util.pad_to_width(label, cell_width)
    end, col_labels)
    lines[#lines + 1] = compose.text("  " .. table.concat(padded, CELL_GAP))
  end
  return compose.col(lines)
end

---@type games.GridWidget
local widget = setmetatable(M, {
  __call = function(_, props)
    return render(props)
  end,
})
return widget
