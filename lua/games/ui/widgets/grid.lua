local art = require("games.ui.art")
local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local predicates = require("games.core.predicates")
local text_util = require("games.ui.text")
local types = require("games.types")

local Group = highlights.Group

---@class games.GridCell
---@field [1] string key into GridProps.art, or the glyph itself without art
---@field [2]? games.Hl foreground layered last, over shade and mark; without it the cell takes the ink of its background

---@alias games.GridLabelList string|string[] a string labels one character per index

---A board cell on the wire, 1-based: row 1 at the top, col 1 at the left.
---@class games.Cell: games.GridPos
---@field row integer
---@field col integer

---@class games.GridMark: games.Cell
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
---@field row_key? fun(row: integer): string names everything cell gives on the row, so that with kept a row whose key is unchanged is not drawn again; not with edges
---@field kept? games.GridRows where the rows that row_key names are kept between draws

---Drawn rows by the grid geometry they were drawn at, then by display row.
---@alias games.GridRows table<string, table<integer, games.GridRow>>

---@class (private) games.GridRow
---@field key string the row_key it was drawn for, with its label
---@field block games.BlockNode

local LIGHT_SQUARE_HL = Group.SQUARE_LIGHT
local DARK_SQUARE_HL = Group.SQUARE_DARK
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
local SEPARATOR_HL = Group.MUTED
local SEPARATOR_WIDTH = 1
---@type games.ArtEntry[]
local UNICODE_SEPARATOR_GLYPHS = vim.tbl_map(function(separator)
  return { h = 1, w = SEPARATOR_WIDTH, lines = { separator } }
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
---@field cell_index fun(items: any[], row_of?: (fun(item: any): integer), col_of?: (fun(item: any): integer)): table<integer, table<integer, any>>
---@field mark fun(row: integer, col: integer, hl: games.Hl): games.GridMark
---@field cell_marks fun(cells: games.Cell[], hl: games.Hl): games.GridMark[]
---@field marks_where fun(rows: any[][], hl_of: fun(value: any, row: integer, col: integer): games.Hl|nil): games.GridMark[]
---@field sample fun(ctx: games.DescribeCtx, cell: games.GridCell, mark_hl?: games.Hl): games.TextNode
---@field samples fun(ctx: games.DescribeCtx, cells: games.GridCell[], mark_hl?: games.Hl): games.TextNode[]
---@field legend_entry fun(ctx: games.DescribeCtx, cell: games.GridCell, meaning: string, mark_hl?: games.Hl): games.LegendEntry
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
  local list = type(labels) == "string" and text_util.chars(labels) or labels
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
---@return string[] str on each of height lines
local function repeated(str, height)
  local lines = {}
  for i = 1, height do
    lines[i] = str
  end
  return lines
end

---One display row of a grid being drawn, a segment at a time, every segment as tall as it.
---@class (private) games.GridLine
---@field parts string[][] by line
---@field bytes integer[] by line, bytes appended so far
---@field last table<integer, integer> by line, the index of its last mark in marks
---@field marks games.Marks
---@field w integer display columns appended so far

---@param height integer
---@return games.GridLine
local function new_line(height)
  local line = { parts = {}, bytes = {}, last = {}, marks = compose.no_marks(), w = 0 }
  for i = 1, height do
    line.parts[i], line.bytes[i] = {}, 0
  end
  return line
end

---Appends a segment w display columns wide, drawing lines[i] on line i, all of it in hl; a mark
---that would start where the line's last one ends, in the same highlight, extends that one.
---@param line games.GridLine
---@param lines string[]
---@param w integer
---@param hl? games.Hl
local function append(line, lines, w, hl)
  local marks = line.marks
  for i, parts in ipairs(line.parts) do
    local str = lines[i]
    parts[#parts + 1] = str
    local start = line.bytes[i]
    local stop = start + #str
    line.bytes[i] = stop
    if hl and stop > start then
      local last = line.last[i]
      if last and marks.end_col[last] == start and compose.same_hl(marks.hl[last], hl) then
        marks.end_col[last] = stop
      else
        line.last[i] = compose.add_mark(marks, i - 1, start, stop, hl)
      end
    end
  end
  line.w = line.w + w
end

---@param line games.GridLine
---@return games.BlockNode
local function finish(line)
  return compose.block(line.w, vim.tbl_map(table.concat, line.parts), line.marks)
end

---content, lines of one display width, centred in a box of size box, odd padding going right and
---down: box.h lines, each box.w columns wide unless content is wider.
---@param content string[]
---@param box games.Size
---@return string[]
local function centred(content, box)
  local top = math.max(0, math.floor((box.h - #content) / 2))
  local blank = text_util.spaces(box.w)
  local lines = {}
  for i = 1, box.h do
    local str = content[i - top]
    lines[i] = str and text_util.center_to_width(str, box.w) or blank
  end
  return lines
end

---@param cell games.Cell
---@return integer
local function cell_row(cell)
  return cell.row
end

---@param cell games.Cell
---@return integer
local function cell_col(cell)
  return cell.col
end

---items by the cell each lies on; of two on one cell the later wins.
---@generic T
---@param items T[]
---@param row_of? fun(item: T): integer default: the games.Cell's row
---@param col_of? fun(item: T): integer default: the games.Cell's col
---@return table<integer, table<integer, T>> row -> col -> item
function M.cell_index(items, row_of, col_of)
  row_of, col_of = row_of or cell_row, col_of or cell_col
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

---A mark of hl on each of cells, in order.
---@param cells games.Cell[]
---@param hl games.Hl
---@return games.GridMark[]
function M.cell_marks(cells, hl)
  return vim.tbl_map(function(cell)
    return M.mark(cell.row, cell.col, hl)
  end, cells)
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

---A cell's highlight: its own foreground over its mark over its shade, or, without one, the ink of
---its background.
---@param shade games.Hl|nil
---@param mark_hl games.Hl|nil
---@param own_hl games.Hl|nil
---@return games.Hl|nil
local function cell_hl(shade, mark_hl, own_hl)
  if not (shade or mark_hl) then
    return own_hl
  end
  return own_hl and compose.layers(shade, mark_hl, own_hl)
    or highlights.with_ink(compose.layers(shade, mark_hl))
end

---A cell as its one-line glyph, in the highlight an unshaded grid gives it under mark_hl, for a legend.
---@param ctx games.DescribeCtx
---@param cell games.GridCell
---@param mark_hl? games.Hl
---@return games.TextNode
function M.sample(ctx, cell, mark_hl)
  local key, own_hl = cell[1], cell[2]
  local glyph = ctx.art and art.glyph(ctx.art, key, ctx.ambiwidth) or key
  return compose.text(glyph, cell_hl(nil, mark_hl, own_hl))
end

---Samples of cells, a space apart, each as M.sample draws it.
---@param ctx games.DescribeCtx
---@param cells games.GridCell[]
---@param mark_hl? games.Hl
---@return games.TextNode[]
function M.samples(ctx, cells, mark_hl)
  local nodes = {}
  for i, cell in ipairs(cells) do
    if i > 1 then
      nodes[#nodes + 1] = compose.text(" ")
    end
    nodes[#nodes + 1] = M.sample(ctx, cell, mark_hl)
  end
  return nodes
end

---A legend entry whose sample is cell, as M.sample draws it.
---@param ctx games.DescribeCtx
---@param cell games.GridCell
---@param meaning string
---@param mark_hl? games.Hl
---@return games.LegendEntry
function M.legend_entry(ctx, cell, meaning, mark_hl)
  return { sample = { M.sample(ctx, cell, mark_hl) }, meaning = meaning }
end

---The lines drawing key in a cell of size box; every cell of one grid shares box and ambiwidth.
---@param props games.GridProps
---@param box games.Size
---@param ambiwidth games.Ambiwidth
---@return fun(key: string): string[] built once per key
local function cell_lines(props, box, ambiwidth)
  local by_key = {}
  return function(key)
    local lines = by_key[key]
    if not lines then
      local entry = props.art and art.pick(props.art, key, box, ambiwidth)
      lines = centred(entry and entry.lines or { key }, box)
      by_key[key] = lines
    end
    return lines
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

---Appends each cell: its key drawn over its shade and mark, in its own highlight.
---@param props games.GridProps
---@param geometry games.GridGeometry
---@param ambiwidth games.Ambiwidth
---@param marks table<integer, table<integer, games.GridMark>> props.marks by cell
---@return fun(line: games.GridLine, r: integer, c: integer)
local function cell_appender(props, geometry, ambiwidth, marks)
  local lines_of = cell_lines(props, geometry.cell, ambiwidth)
  return function(line, r, c)
    local cell = props.cell(r, c)
    local mark = marks[r] and marks[r][c]
    local hl = cell_hl(shade_hl(props.shade, r, c), mark and mark.hl, cell[2])
    append(line, lines_of(cell[1]), geometry.cell.w, hl)
  end
end

---@param label string
---@param geometry games.GridGeometry
---@param height integer
---@return string[] label left-aligned in the row label column, height lines tall
local function label_lines(label, geometry, height)
  local padded = text_util.pad_to_width(label, geometry.label) .. LABEL_GAP
  return centred({ padded }, { w = geometry.prefix, h = height })
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

---The rows props.kept holds for a grid of this geometry, or nil when it keeps none.
---@param props games.GridProps
---@param geometry games.GridGeometry
---@param ambiwidth games.Ambiwidth
---@return table<integer, games.GridRow>|nil
local function kept_rows(props, geometry, ambiwidth)
  if not (props.kept and props.row_key) then
    return nil
  end
  local drawn_at = table.concat({
    props.rows,
    props.cols,
    geometry.cell.w,
    geometry.cell.h,
    geometry.gap,
    geometry.prefix,
    ambiwidth,
    props.shade or "",
    props.blocks or "",
    props.glyphs or "",
    tostring(props.flip),
    tostring(props.art),
  }, " ")
  props.kept[drawn_at] = props.kept[drawn_at] or {}
  return props.kept[drawn_at]
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
  ---@type table<integer, table<integer, games.GridMark>>
  local marks = M.cell_index(props.marks or {})
  local append_cell = cell_appender(props, geometry, ambiwidth, marks)
  local separator_lines = repeated(separators.col, cell_height)
  local gap_lines = repeated(text_util.spaces(geometry.gap), cell_height)

  ---@param i integer display row
  ---@param r integer logical row
  ---@return games.BlockNode
  local function draw_row(i, r)
    local line = new_line(cell_height)
    if row_labels then
      append(line, label_lines(row_labels[i], geometry, cell_height), geometry.prefix)
    end
    for j, c in ipairs(col_order) do
      if j > 1 and crosses_block(props.blocks, col_order[j - 1], c) then
        append(line, separator_lines, SEPARATOR_WIDTH, SEPARATOR_HL)
      elseif j > 1 and geometry.gap > 0 then
        append(line, gap_lines, geometry.gap)
      end
      append_cell(line, r, c)
    end
    return finish(line)
  end

  local kept = kept_rows(props, geometry, ambiwidth)
  ---Display row i, logical row r, as kept when its key is unchanged.
  ---@param i integer
  ---@param r integer
  ---@return games.BlockNode
  local function row_block(i, r)
    if not kept or marks[r] then
      return draw_row(i, r)
    end
    local key = (row_labels and row_labels[i] or "") .. props.row_key(r)
    local row = kept[i]
    if not (row and row.key == key) then
      row = { key = key, block = draw_row(i, r) }
      kept[i] = row
    end
    return row.block
  end

  local lines = {}
  for i, r in ipairs(row_order) do
    if i > 1 and crosses_block(props.blocks, row_order[i - 1], r) then
      lines[#lines + 1] = block_separator_row(props, col_order, geometry, separators)
    end
    lines[#lines + 1] = row_block(i, r)
  end

  if col_labels then
    lines[#lines + 1] = column_label_row(props, col_labels, col_order, geometry)
  end
  return compose.col(lines)
end

---An edge or dot glyph, as lines for each segment it draws.
---@class (private) games.GridEdgeGlyph
---@field w integer display columns of one glyph
---@field dot string[] the glyph alone
---@field across string[] the glyph along a cell's width
---@field down string[] the glyph down a cell's height

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
  local append_cell = cell_appender(props, geometry, ambiwidth, M.cell_index(props.marks or {}))

  ---@type table<string, games.GridEdgeGlyph>
  local glyphs = {}
  ---@param key string
  ---@return games.GridEdgeGlyph
  local function glyph(key)
    if not glyphs[key] then
      local entry = props.art and art.glyph_entry(props.art, key, ambiwidth)
      local str = entry and entry.lines[1] or key
      glyphs[key] = {
        w = entry and entry.w or text_util.width(key),
        dot = { str },
        across = { str:rep(cell_w) },
        down = repeated(str, cell_h),
      }
    end
    return glyphs[key]
  end
  ---@param line games.GridLine
  ---@param cell games.GridCell
  ---@param segment "dot"|"across"|"down"
  ---@param repeats integer glyphs across the segment
  local function append_edge(line, cell, segment, repeats)
    local edge = glyph(cell[1])
    append(line, edge[segment], edge.w * repeats, cell[2])
  end

  local lines = {}
  for r = 1, props.rows + 1 do
    local corners = new_line(1)
    if row_labels then
      append(corners, label_lines(row_labels[r], geometry, 1), geometry.prefix)
    end
    append_edge(corners, edges.dot(r, 1), "dot", 1)
    for c = 1, props.cols do
      append_edge(corners, edges.across(r, c), "across", cell_w)
      append_edge(corners, edges.dot(r, c + 1), "dot", 1)
    end
    lines[#lines + 1] = finish(corners)
    if r <= props.rows then
      local sides = new_line(cell_h)
      if row_labels then
        append(sides, label_lines("", geometry, cell_h), geometry.prefix)
      end
      append_edge(sides, edges.down(r, 1), "down", 1)
      for c = 1, props.cols do
        append_cell(sides, r, c)
        append_edge(sides, edges.down(r, c + 1), "down", 1)
      end
      lines[#lines + 1] = finish(sides)
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
