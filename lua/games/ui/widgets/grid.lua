local art = require("games.ui.art")
local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local predicates = require("games.core.predicates")
local text_util = require("games.ui.text")
local types = require("games.types")

---@class games.GridCell
---@field [1] string key into GridProps.art, or the glyph itself without art
---@field [2]? games.Hl foreground layered last, over shade and mark; without it the cell takes the ink of its background

---@alias games.GridLabelList string|string[] a string labels one character per index

---@class games.GridMark
---@field row integer logical row
---@field col integer logical column
---@field hl games.Hl layered over the cell

---@class games.GridLabels
---@field cols games.GridLabelList
---@field rows games.GridLabelList

---Lines one column or one line thick along every side of every cell, with a dot at every corner.
---Each callback gives the asset key of a one-column glyph, repeated along the edge, and its
---highlight; a dot or edge without a highlight takes none.
---@class games.GridEdges
---@field across fun(row: integer, col: integer): games.GridCell the edge from dot (row, col) to the dot on its right; rows + 1 rows of cols
---@field down fun(row: integer, col: integer): games.GridCell the edge from dot (row, col) to the dot below; rows rows of cols + 1
---@field dot fun(row: integer, col: integer): games.GridCell rows + 1 rows of cols + 1

---@class games.GridProps
---@field rows integer
---@field cols integer
---@field cell fun(row: integer, col: integer): games.GridCell
---@field shade? "checker"|"plain" default "plain"
---@field labels? games.GridLabels
---@field marks? games.GridMark[] a later mark on a cell replaces an earlier one
---@field flip? boolean
---@field cell_width? integer display columns per cell, default 1
---@field cell_height? integer lines per cell, default 1; taller cells touch, with no gap
---@field gap? integer display columns between cells one line tall, default 1; 0 makes them touch
---@field art? games.ArtSet resolves cell keys; without it a key is drawn as it is
---@field ambiwidth? games.Ambiwidth the 'ambiwidth' in effect, default "single"
---@field blocks? integer positive: draw a thin separator after every this many rows and columns
---@field glyphs? games.Glyphs separator characters, default "unicode"
---@field edges? games.GridEdges draws the cells' edges and corners in place of the gaps between cells; labels then name the dots, rows + 1 and cols + 1 of them; not with flip or blocks

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
---@type games.ArtEntry[]
local UNICODE_SEPARATOR_GLYPHS = vim.tbl_map(function(separator)
  return { h = 1, w = 1, lines = { separator } }
end, vim.tbl_values(BLOCK_SEPARATORS.unicode))

---The separators of glyph mode, or ascii ones where unicode box drawing would widen.
---@param glyphs games.Glyphs
---@param ambiwidth games.Ambiwidth
---@return games.BlockSeparators
local function separators_for(glyphs, ambiwidth)
  local drawable = vim.iter(UNICODE_SEPARATOR_GLYPHS):all(function(entry)
    return art.drawable(entry, ambiwidth)
  end)
  return BLOCK_SEPARATORS[drawable and glyphs or types.Glyphs.ASCII]
end

---@class games.GridWidget
---@field is_dark fun(row: integer, col: integer): boolean
---@field display_order fun(n: integer, flip: boolean|nil): integer[]
---@field cell_index fun(items: any[], row_of: fun(item: any): integer, col_of: fun(item: any): integer): table<integer, table<integer, any>>
---@field mark fun(row: integer, col: integer, hl: games.Hl): games.GridMark
---@field marks_where fun(rows: any[][], hl_of: fun(value: any, row: integer, col: integer): games.Hl|nil): games.GridMark[]
---@overload fun(props: games.GridProps): games.Node

local M = {}

---Whether the cell at logical row and col takes the dark shade of a checker grid.
---@param row integer
---@param col integer
---@return boolean
function M.is_dark(row, col)
  return (row + col) % 2 == 1
end

---@param shade "checker"|"plain"|nil
---@param row integer
---@param col integer
---@return string|nil
local function shade_hl(shade, row, col)
  if shade ~= "checker" then
    return nil
  end
  return M.is_dark(row, col) and DARK_SQUARE_HL or LIGHT_SQUARE_HL
end

---The logical indices 1..n in the order the grid draws them: reversed when flipped.
---@param n integer
---@param flip boolean|nil
---@return integer[]
function M.display_order(n, flip)
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
  if not (blocks == nil or predicates.is_positive_integer(blocks)) then
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

---items by the cell each lies on; of two on one cell the later wins.
---@generic T
---@param items T[]
---@param row_of fun(item: T): integer
---@param col_of fun(item: T): integer
---@return table<integer, table<integer, T>> row -> col -> item
function M.cell_index(items, row_of, col_of)
  local by_cell = {}
  for _, item in ipairs(items) do
    local row = row_of(item)
    by_cell[row] = by_cell[row] or {}
    by_cell[row][col_of(item)] = item
  end
  return by_cell
end

---@param row integer
---@param col integer
---@param hl games.Hl
---@return games.GridMark
function M.mark(row, col, hl)
  return { row = row, col = col, hl = hl }
end

---A mark on each cell of rows that hl_of gives a highlight, in row then column order.
---@generic T
---@param rows T[][] by row, then column
---@param hl_of fun(value: T, row: integer, col: integer): games.Hl|nil
---@return games.GridMark[]
function M.marks_where(rows, hl_of)
  local marks = {}
  for row, values in ipairs(rows) do
    for col, value in ipairs(values) do
      local hl = hl_of(value, row, col)
      if hl then
        marks[#marks + 1] = M.mark(row, col, hl)
      end
    end
  end
  return marks
end

---@param mark games.GridMark
---@return integer
local function row_of(mark)
  return mark.row
end

---@param mark games.GridMark
---@return integer
local function col_of(mark)
  return mark.col
end

---The node drawing key in a cell of size box; every cell of one grid shares box and ambiwidth.
---@param props games.GridProps
---@param box games.Size
---@param ambiwidth games.Ambiwidth
---@return fun(key: string): games.Node built once per key
local function cell_contents(props, box, ambiwidth)
  local by_key = {}
  return function(key)
    local content = by_key[key]
    if not content then
      content = props.art
          and compose.col(vim.tbl_map(compose.text, art.pick(props.art, key, box, ambiwidth).lines))
        or compose.text(key)
      by_key[key] = content
    end
    return content
  end
end

---@param props games.GridProps
---@param col_order integer[]
---@param geometry games.GridGeometry
---@param separators games.BlockSeparators
---@return games.Node
local function block_separator_row(props, col_order, geometry, separators)
  local cell = separators.row:rep(geometry.cell.w)
  local parts = { text_util.spaces(geometry.prefix), cell }
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
  local parts = { text_util.spaces(geometry.prefix) }
  for j, label in ipairs(col_labels) do
    if j > 1 then
      local crossing = crosses_block(props.blocks, col_order[j - 1], col_order[j])
      parts[#parts + 1] = text_util.spaces(crossing and 1 or geometry.gap)
    end
    parts[#parts + 1] = text_util.center_to_width(label, geometry.cell.w)
  end
  return compose.text(table.concat(parts))
end

---The node of each cell: its key drawn over its shade and mark, in its own highlight.
---@param props games.GridProps
---@param geometry games.GridGeometry
---@param ambiwidth games.Ambiwidth
---@return fun(r: integer, c: integer): games.Node
local function cell_nodes(props, geometry, ambiwidth)
  ---@type table<integer, table<integer, games.GridMark>>
  local marks = M.cell_index(props.marks or {}, row_of, col_of)
  local content = cell_contents(props, geometry.cell, ambiwidth)
  return function(r, c)
    local key, own_hl = unpack(props.cell(r, c))
    local mark = marks[r] and marks[r][c]
    local shade, mark_hl = shade_hl(props.shade, r, c), mark and mark.hl
    local hl = own_hl and compose.layers(shade, mark_hl, own_hl)
      or highlights.with_ink(compose.layers(shade, mark_hl))
    return compose.box({ w = geometry.cell.w, h = geometry.cell.h, hl = hl, child = content(key) })
  end
end

---@param label string
---@param geometry games.GridGeometry
---@param height integer
---@return games.Node label left-aligned in the row label column, height lines tall
local function label_node(label, geometry, height)
  local padded = text_util.pad_to_width(label, geometry.label) .. LABEL_GAP
  return compose.box({ w = geometry.prefix, h = height, child = compose.text(padded) })
end

---@param props games.GridProps
---@param row_labels string[]|nil
---@return games.GridGeometry
local function geometry_of(props, row_labels)
  local label_width = row_labels and text_util.max_display_width(row_labels) or 0
  local cell_height = props.cell_height or 1
  return {
    cell = { w = props.cell_width or 1, h = cell_height },
    gap = cell_height == 1 and (props.gap or #CELL_GAP) or 0,
    prefix = row_labels and label_width + #LABEL_GAP or 0,
    label = label_width,
  }
end

---@param props games.GridProps
---@param ambiwidth games.Ambiwidth
---@return games.Node
local function render_cells(props, ambiwidth)
  local separators = separators_for(props.glyphs or types.Glyphs.UNICODE, ambiwidth)
  local row_order = M.display_order(props.rows, props.flip)
  local col_order = M.display_order(props.cols, props.flip)
  local row_labels = props.labels and labels_in_order(props.labels.rows, row_order)
  local col_labels = props.labels and labels_in_order(props.labels.cols, col_order)
  local geometry = geometry_of(props, row_labels)
  local cell_height = geometry.cell.h
  local cell_node = cell_nodes(props, geometry, ambiwidth)

  local function gap_node(j)
    if crosses_block(props.blocks, col_order[j - 1], col_order[j]) then
      return repeated(separators.col, cell_height, SEPARATOR_HL)
    end
    return geometry.gap > 0 and repeated(text_util.spaces(geometry.gap), cell_height) or nil
  end

  local lines = {}
  for i, r in ipairs(row_order) do
    if i > 1 and crosses_block(props.blocks, row_order[i - 1], r) then
      lines[#lines + 1] = block_separator_row(props, col_order, geometry, separators)
    end
    local parts = {}
    if row_labels then
      parts[#parts + 1] = label_node(row_labels[i], geometry, cell_height)
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

---The cells with a dot at every corner and an edge along every side, labels naming the dots.
---@param props games.GridProps
---@param edges games.GridEdges
---@param ambiwidth games.Ambiwidth
---@return games.Node
local function render_lattice(props, edges, ambiwidth)
  if props.flip or props.blocks then
    error("grid: edges draw neither flipped nor in blocks", 3)
  end
  local dot_rows, dot_cols = M.display_order(props.rows + 1), M.display_order(props.cols + 1)
  local row_labels = props.labels and labels_in_order(props.labels.rows, dot_rows)
  local col_labels = props.labels and labels_in_order(props.labels.cols, dot_cols)
  local geometry = geometry_of(props, row_labels)
  local cell_w, cell_h = geometry.cell.w, geometry.cell.h
  local cell_node = cell_nodes(props, geometry, ambiwidth)

  local function glyph(key)
    return props.art and art.glyph(props.art, key, ambiwidth) or key
  end
  local function across(r, c)
    local key, hl = unpack(edges.across(r, c))
    return compose.text(glyph(key):rep(cell_w), hl)
  end
  local function down(r, c)
    local key, hl = unpack(edges.down(r, c))
    return repeated(glyph(key), cell_h, hl)
  end
  local function dot(r, c)
    local key, hl = unpack(edges.dot(r, c))
    return compose.text(glyph(key), hl)
  end

  local lines = {}
  for r = 1, props.rows + 1 do
    local corners = row_labels and { label_node(row_labels[r], geometry, 1) } or {}
    corners[#corners + 1] = dot(r, 1)
    for c = 1, props.cols do
      vim.list_extend(corners, { across(r, c), dot(r, c + 1) })
    end
    lines[#lines + 1] = compose.row(corners)
    if r <= props.rows then
      local sides = row_labels and { label_node("", geometry, cell_h) } or {}
      sides[#sides + 1] = down(r, 1)
      for c = 1, props.cols do
        vim.list_extend(sides, { cell_node(r, c), down(r, c + 1) })
      end
      lines[#lines + 1] = compose.row(sides)
    end
  end

  if col_labels then
    local between_dots =
      { prefix = geometry.prefix, cell = { w = 1, h = 1 }, gap = cell_w, label = 0 }
    lines[#lines + 1] = column_label_row(props, col_labels, dot_cols, between_dots)
  end
  return compose.col(lines)
end

---@param props games.GridProps
---@return games.Node
local function render(props)
  validate_blocks(props.blocks)
  local ambiwidth = props.ambiwidth or types.DEFAULT_AMBIWIDTH
  if props.edges then
    return render_lattice(props, props.edges, ambiwidth)
  end
  return render_cells(props, ambiwidth)
end

---@type games.GridWidget
local widget = setmetatable(M, {
  __call = function(_, props)
    return render(props)
  end,
})
return widget
