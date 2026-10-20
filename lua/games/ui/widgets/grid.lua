local art = require("games.ui.art")
local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local text_util = require("games.ui.text")

---@class games.GridCell
---@field [1] string key into GridProps.art, or the glyph itself without art
---@field [2]? games.Hl foreground layered last, over shade and mark; without it the cell takes the ink of its background

---@alias games.GridLabelList string|string[] a string labels one character per index

---@class games.GridLabels
---@field cols games.GridLabelList
---@field rows games.GridLabelList

---@class games.GridProps
---@field rows integer
---@field cols integer
---@field cell fun(row: integer, col: integer): games.GridCell
---@field shade "checker"|"plain"
---@field labels? games.GridLabels
---@field marks? table<string, games.Hl> mark_key(row, col) -> highlight layered over the cell
---@field flip? boolean
---@field cell_width? integer display columns per cell, default 1
---@field cell_height? integer lines per cell, default 1; taller cells touch, with no gap
---@field art? games.ArtSet resolves cell keys; without it a key is drawn as it is
---@field blocks? integer positive: draw a thin separator after every this many rows and columns
---@field glyphs? games.Glyphs separator characters, default "unicode"

local LIGHT_SQUARE_HL = "GamesSquareLight"
local DARK_SQUARE_HL = "GamesSquareDark"
local CELL_GAP = " "
local LABEL_GAP = " "

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
---@field is_dark fun(row: integer, col: integer): boolean
---@overload fun(props: games.GridProps): games.Node

local M = {}

---Key of the cell at logical row and col in GridProps.marks.
---@param row integer
---@param col integer
---@return string
function M.mark_key(row, col)
  return row .. "," .. col
end

---Whether the cell at logical row and col takes the dark shade of a checker grid.
---@param row integer
---@param col integer
---@return boolean
function M.is_dark(row, col)
  return (row + col) % 2 == 1
end

---@param shade "checker"|"plain"
---@param row integer
---@param col integer
---@return string|nil
local function shade_hl(shade, row, col)
  if shade ~= "checker" then
    return nil
  end
  return M.is_dark(row, col) and DARK_SQUARE_HL or LIGHT_SQUARE_HL
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

---@param labels games.GridLabelList
---@param order integer[]
---@return string[]
local function labels_in_order(labels, order)
  local list = type(labels) == "string" and vim.fn.split(labels, "\\zs") or labels
  return vim.tbl_map(function(i)
    return list[i]
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

---@param blocks integer|nil
local function validate_blocks(blocks)
  local valid = blocks == nil
    or (type(blocks) == "number" and blocks >= 1 and blocks == math.floor(blocks))
  if not valid then
    error(("grid: blocks must be a positive integer or nil, got %s"):format(vim.inspect(blocks)), 2)
  end
end

---@class (private) games.GridGeometry
---@field cell games.Size
---@field gap integer display columns between cells that no block separator divides
---@field prefix integer display width of the row label column
---@field label integer display width of the widest row label

---@param str string
---@param height integer
---@param hl? games.Hl
---@return games.Node str on each of height lines
local function repeated(str, height, hl)
  local lines = {}
  for i = 1, height do
    lines[i] = compose.text(str, hl)
  end
  return compose.col(lines)
end

---@param props games.GridProps
---@param key string
---@param box games.Size
---@return games.Node
local function cell_content(props, key, box)
  if not props.art then
    return compose.text(key)
  end
  return compose.col(vim.tbl_map(compose.text, art.pick(props.art, key, box).lines))
end

---@param props games.GridProps
---@param col_order integer[]
---@param geometry games.GridGeometry
---@param separators games.BlockSeparators
---@return games.Node
local function block_separator_row(props, col_order, geometry, separators)
  local cell = separators.row:rep(geometry.cell.w)
  local parts = { (" "):rep(geometry.prefix), cell }
  for j = 2, #col_order do
    local crossing = crosses_block(props.blocks, col_order[j - 1], col_order[j])
    parts[#parts + 1] = crossing and separators.crossing or separators.row:rep(geometry.gap)
    parts[#parts + 1] = cell
  end
  return compose.text(table.concat(parts), SEPARATOR_HL)
end

---@param props games.GridProps
---@param col_labels string[]
---@param col_order integer[]
---@param geometry games.GridGeometry
---@return games.Node
local function column_label_row(props, col_labels, col_order, geometry)
  local parts = { (" "):rep(geometry.prefix) }
  for j, label in ipairs(col_labels) do
    if j > 1 then
      local crossing = crosses_block(props.blocks, col_order[j - 1], col_order[j])
      parts[#parts + 1] = (" "):rep(crossing and 1 or geometry.gap)
    end
    parts[#parts + 1] = text_util.center_to_width(label, geometry.cell.w)
  end
  return compose.text(table.concat(parts))
end

---@param props games.GridProps
---@return games.Node
local function render(props)
  validate_blocks(props.blocks)
  local separators = BLOCK_SEPARATORS[props.glyphs or "unicode"]
  local cell_height = props.cell_height or 1
  local marks = props.marks or {}
  local row_order = display_order(props.rows, props.flip == true)
  local col_order = display_order(props.cols, props.flip == true)
  local row_labels = props.labels and labels_in_order(props.labels.rows, row_order)
  local col_labels = props.labels and labels_in_order(props.labels.cols, col_order)
  local label_width = row_labels and text_util.max_display_width(row_labels) or 0
  ---@type games.GridGeometry
  local geometry = {
    cell = { w = props.cell_width or 1, h = cell_height },
    gap = cell_height == 1 and #CELL_GAP or 0,
    prefix = row_labels and label_width + #LABEL_GAP or 0,
    label = label_width,
  }

  local function cell_node(r, c)
    local key, own_hl = unpack(props.cell(r, c))
    local shade, mark = shade_hl(props.shade, r, c), marks[M.mark_key(r, c)]
    local hl = own_hl and compose.layers(shade, mark, own_hl)
      or highlights.with_ink(compose.layers(shade, mark))
    local box = geometry.cell
    return compose.box({ w = box.w, h = box.h, hl = hl, child = cell_content(props, key, box) })
  end

  local function gap_node(j)
    if crosses_block(props.blocks, col_order[j - 1], col_order[j]) then
      return repeated(separators.col, cell_height, SEPARATOR_HL)
    end
    return geometry.gap > 0 and repeated(CELL_GAP, cell_height) or nil
  end

  local function label_node(label)
    local padded = text_util.pad_to_width(label, geometry.label) .. LABEL_GAP
    return compose.box({ w = geometry.prefix, h = cell_height, child = compose.text(padded) })
  end

  local lines = {}
  for i, r in ipairs(row_order) do
    if i > 1 and crosses_block(props.blocks, row_order[i - 1], r) then
      lines[#lines + 1] = block_separator_row(props, col_order, geometry, separators)
    end
    local parts = {}
    if row_labels then
      parts[#parts + 1] = label_node(row_labels[i])
    end
    for j, c in ipairs(col_order) do
      parts[#parts + 1] = j > 1 and gap_node(j) or nil
      parts[#parts + 1] = cell_node(r, c)
    end
    lines[#lines + 1] = compose.row(parts)
  end

  if col_labels then
    lines[#lines + 1] = column_label_row(props, col_labels, col_order, geometry)
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
