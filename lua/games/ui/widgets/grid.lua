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
---@field marks? table<{[1]: integer, [2]: integer}, string> logical {row, col} -> highlight group
---@field flip? boolean
---@field cell_width? integer display columns per cell, default 1

local SHADE_BY_PARITY = { [0] = "GamesSquareLight", [1] = "GamesSquareDark" }

---@param marks table<{[1]: integer, [2]: integer}, string>
---@return table<string, string>
local function index_marks(marks)
  local by_position = {}
  for position, hl in pairs(marks) do
    by_position[position[1] .. "," .. position[2]] = hl
  end
  return by_position
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

---@param props games.GridProps
---@return games.Node
return function(props)
  local cell_width = props.cell_width or 1
  local marks = index_marks(props.marks or {})
  local row_order = display_order(props.rows, props.flip == true)
  local col_order = display_order(props.cols, props.flip == true)
  local row_labels = props.labels and labels_in_order(props.labels.rows, row_order)
  local col_labels = props.labels and labels_in_order(props.labels.cols, col_order)

  local function cell_node(r, c)
    local glyph, own_hl = unpack(props.cell(r, c))
    local shade = props.shade == "checker" and SHADE_BY_PARITY[(r + c) % 2] or nil
    local hl = marks[r .. "," .. c] or own_hl or shade
    return compose.text(text_util.pad_to_width(glyph, cell_width), hl)
  end

  local lines = {}
  for i, r in ipairs(row_order) do
    local parts = {}
    if row_labels then
      parts[#parts + 1] = compose.text(row_labels[i] .. " ")
    end
    for j, c in ipairs(col_order) do
      if j > 1 then
        parts[#parts + 1] = compose.text(" ")
      end
      parts[#parts + 1] = cell_node(r, c)
    end
    lines[#lines + 1] = compose.row(parts)
  end

  if col_labels then
    local padded = vim.tbl_map(function(label)
      return text_util.pad_to_width(label, cell_width)
    end, col_labels)
    lines[#lines + 1] = compose.text("  " .. table.concat(padded, " "))
  end
  return compose.col(lines)
end
