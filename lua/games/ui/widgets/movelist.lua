local compose = require("games.ui.compose")

---@class games.MovelistProps
---@field items string[]
---@field columns integer items per line
---@field numbered boolean prefix each line with its 1-based line number

---@param props games.MovelistProps
---@return games.Node
return function(props)
  local columns = props.columns
  local column_widths = {}
  for i, item in ipairs(props.items) do
    local column = (i - 1) % columns + 1
    column_widths[column] = math.max(column_widths[column] or 0, vim.fn.strdisplaywidth(item))
  end

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
        local gap = (" "):rep(column_widths[column] - vim.fn.strdisplaywidth(item))
        cells[#cells + 1] = item .. gap
      end
    end
    rows[line] = compose.text((table.concat(cells, " "):gsub("%s+$", "")))
  end
  return compose.col(rows)
end
