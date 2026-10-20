local compose = require("games.ui.compose")
local text_util = require("games.ui.text")

---@class games.MovelistProps
---@field items string[]
---@field columns integer items per line
---@field numbered boolean prefix each line with its 1-based line number

---@param props games.MovelistProps
---@return games.Node
return function(props)
  local columns = props.columns
  local items_by_column = {}
  for i, item in ipairs(props.items) do
    local column = (i - 1) % columns + 1
    items_by_column[column] = items_by_column[column] or {}
    table.insert(items_by_column[column], item)
  end
  local column_widths = vim.tbl_map(text_util.max_display_width, items_by_column)

  local line_count = math.ceil(#props.items / columns)
  local number_width = #tostring(line_count)
  local rows = {}
  for line = 1, line_count do
    local cells = {}
    if props.numbered then
      cells[1] = ("%" .. number_width .. "d."):format(line)
    end
    for column = 1, columns do
      local item = props.items[(line - 1) * columns + column]
      if item then
        cells[#cells + 1] = text_util.pad_to_width(item, column_widths[column])
      end
    end
    rows[line] = compose.text((table.concat(cells, " "):gsub("%s+$", "")))
  end
  return compose.col(rows)
end
