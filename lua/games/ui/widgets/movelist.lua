local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local text_util = require("games.ui.text")

local NUMBER_HL = "GamesMuted"
local LATEST_HL = highlights.with_ink("GamesLastMove")
local SKIPPED = "..."

---@class games.MoveAnnotation
---@field pattern string Lua pattern for the annotation that ends an item, e.g. "[+#]$"
---@field hl games.Hl

---@class games.MovelistProps
---@field items string[]
---@field columns integer items per line
---@field numbered? boolean prefix each line with its 1-based line number
---@field annotation? games.MoveAnnotation highlighted wherever it ends an item
---@field max_lines? integer show only the last this many lines
---@field offset? integer places the numbering counts before the first item, such as the plies before a black-first line; the first line marks its skipped places with ...

---@param item string
---@param annotation games.MoveAnnotation|nil
---@param hl games.Hl|nil the whole item's highlight
---@return games.Node[]
local function item_nodes(item, annotation, hl)
  local start = annotation and item:find(annotation.pattern)
  if not (annotation and start) then
    return { compose.text(item, hl) }
  end
  return {
    compose.text(item:sub(1, start - 1), hl),
    compose.text(item:sub(start), compose.layers(hl, annotation.hl)),
  }
end

---@class games.MovelistPanelSpec
---@field items string[]
---@field columns integer items per line
---@field min_width integer display columns the panel keeps however short the list
---@field annotation? games.MoveAnnotation
---@field offset? integer as in games.MovelistProps

---@class games.MovelistWidget
---@field panel fun(spec: games.MovelistPanelSpec): games.BoardPanel
---@overload fun(props: games.MovelistProps): games.Node

local M = {}

---@param props games.MovelistProps
---@return games.Node
local function render(props)
  local columns = props.columns
  local offset = props.offset or 0
  local skipped = offset % columns
  local first_number = (offset - skipped) / columns + 1
  local slots = {}
  for _ = 1, skipped do
    slots[#slots + 1] = SKIPPED
  end
  vim.list_extend(slots, props.items)
  local slots_by_column = vim.defaulttable()
  for i, slot in ipairs(slots) do
    table.insert(slots_by_column[(i - 1) % columns + 1], slot)
  end
  local column_widths = vim.tbl_map(text_util.max_display_width, slots_by_column)

  local line_count = math.ceil(#slots / columns)
  local number_width = #tostring(first_number + line_count - 1)
  local first_line = math.max(1, line_count - (props.max_lines or line_count) + 1)
  local rows = {}
  for line = first_line, line_count do
    local parts = {}
    if props.numbered then
      parts[1] =
        compose.text(("%" .. number_width .. "d."):format(first_number + line - 1), NUMBER_HL)
    end
    local first = (line - 1) * columns + 1
    local last = math.min(first + columns - 1, #slots)
    for i = first, last do
      local slot = slots[i]
      if #parts > 0 then
        parts[#parts + 1] = compose.text(" ")
      end
      if i <= skipped then
        parts[#parts + 1] = compose.text(slot, NUMBER_HL)
      else
        vim.list_extend(parts, item_nodes(slot, props.annotation, i == #slots and LATEST_HL or nil))
      end
      if i < last then
        local column_width = column_widths[(i - 1) % columns + 1]
        parts[#parts + 1] =
          compose.text(text_util.spaces(column_width - vim.fn.strdisplaywidth(slot)))
      end
    end
    rows[#rows + 1] = compose.row(parts)
  end
  return compose.col(rows)
end

---A numbered move list as a board panel, min_width wide unless the whole list needs more,
---so an ordinary game never resizes the board; it shows the last lines that fit.
---@param spec games.MovelistPanelSpec
---@return games.BoardPanel
function M.panel(spec)
  ---@param max_lines? integer
  ---@return games.Node
  local function node(max_lines)
    return render({
      items = spec.items,
      columns = spec.columns,
      numbered = true,
      annotation = spec.annotation,
      offset = spec.offset,
      max_lines = max_lines,
    })
  end
  return { width = math.max(spec.min_width, compose.measure(node()).w), node = node }
end

---@type games.MovelistWidget
local widget = setmetatable(M, {
  __call = function(_, props)
    return render(props)
  end,
})
return widget
