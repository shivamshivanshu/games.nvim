local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local text_util = require("games.ui.text")

local NUMBER_HL = "GamesMuted"
local LATEST_HL = highlights.with_ink("GamesLastMove")

---@class games.MoveAnnotation
---@field pattern string Lua pattern for the annotation that ends an item, e.g. "[+#]$"
---@field hl games.Hl

---@class games.MovelistProps
---@field items string[]
---@field columns integer items per line
---@field numbered? boolean prefix each line with its 1-based line number
---@field annotation? games.MoveAnnotation highlighted wherever it ends an item

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

---@param props games.MovelistProps
---@return games.Node
return function(props)
  local columns = props.columns
  local items_by_column = vim.defaulttable()
  for i, item in ipairs(props.items) do
    table.insert(items_by_column[(i - 1) % columns + 1], item)
  end
  local column_widths = vim.tbl_map(text_util.max_display_width, items_by_column)

  local line_count = math.ceil(#props.items / columns)
  local number_width = #tostring(line_count)
  local rows = {}
  for line = 1, line_count do
    local parts = {}
    if props.numbered then
      parts[1] = compose.text(("%" .. number_width .. "d."):format(line), NUMBER_HL)
    end
    local first = (line - 1) * columns + 1
    local last = math.min(first + columns - 1, #props.items)
    for i = first, last do
      local item = props.items[i]
      if #parts > 0 then
        parts[#parts + 1] = compose.text(" ")
      end
      vim.list_extend(
        parts,
        item_nodes(item, props.annotation, i == #props.items and LATEST_HL or nil)
      )
      if i < last then
        local column_width = column_widths[(i - 1) % columns + 1]
        parts[#parts + 1] = compose.text((" "):rep(column_width - vim.fn.strdisplaywidth(item)))
      end
    end
    rows[line] = compose.row(parts)
  end
  return compose.col(rows)
end
