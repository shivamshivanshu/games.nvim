---@class games.Paint
---@field with_modifiable fun(buf: integer, fn: fun())
---@field update fun(buf: integer, ns: integer, old: games.Painted|nil, new: games.Rendered): games.Painted
---@overload fun(buf: integer, ns: integer, first: integer, last: integer, rendered: games.Rendered)

---What paint.update drew into a buffer, which still shows it while its changedtick is unchanged.
---@class games.Painted
---@field rendered games.Rendered
---@field changedtick integer

local M = {}

---@type games.Extmark[]
local NO_MARKS = {}

---Runs fn on a read-only buf made modifiable for it, read-only again even when fn raises.
---@param buf integer
---@param fn fun()
function M.with_modifiable(buf, fn)
  vim.bo[buf].modifiable = true
  local ok, err = pcall(fn)
  vim.bo[buf].modifiable = false
  if not ok then
    error(err, 0)
  end
end

---@param buf integer
---@param ns integer
---@param first integer line the marks' rows count from
---@param extmarks games.Extmark[]
local function set_marks(buf, ns, first, extmarks)
  for _, mark in ipairs(extmarks) do
    vim.api.nvim_buf_set_extmark(buf, ns, first + mark.row, mark.col, {
      end_col = mark.end_col,
      hl_group = mark.hl,
    })
  end
end

---Replaces lines [first, last) of a read-only buf with rendered, highlighting it in namespace ns.
---@param buf integer
---@param ns integer
---@param first integer
---@param last integer exclusive, -1 for the end of the buffer
---@param rendered games.Rendered
local function paint(buf, ns, first, last, rendered)
  vim.api.nvim_buf_clear_namespace(buf, ns, first, last)
  M.with_modifiable(buf, function()
    vim.api.nvim_buf_set_lines(buf, first, last, false, rendered.lines)
  end)
  set_marks(buf, ns, first, rendered.extmarks)
end

---@param a games.Hl
---@param b games.Hl
---@return boolean
local function same_hl(a, b)
  if type(a) ~= "table" or type(b) ~= "table" then
    return a == b
  end
  if #a ~= #b then
    return false
  end
  for i = 1, #a do
    if a[i] ~= b[i] then
      return false
    end
  end
  return true
end

---@param a games.Extmark[]
---@param b games.Extmark[]
---@return boolean
local function same_marks(a, b)
  if #a ~= #b then
    return false
  end
  for i = 1, #a do
    if a[i].col ~= b[i].col or a[i].end_col ~= b[i].end_col or not same_hl(a[i].hl, b[i].hl) then
      return false
    end
  end
  return true
end

---@param extmarks games.Extmark[]
---@return table<integer, games.Extmark[]> by row, in their order, holding only rows that have marks
local function marks_by_row(extmarks)
  local by_row = {}
  for _, mark in ipairs(extmarks) do
    local row_marks = by_row[mark.row]
    if not row_marks then
      row_marks = {}
      by_row[mark.row] = row_marks
    end
    row_marks[#row_marks + 1] = mark
  end
  return by_row
end

---Rewrites only the lines of buf whose text or highlights differ between old and new, which have
---the same line count.
---@param buf integer
---@param ns integer
---@param old games.Rendered
---@param new games.Rendered
local function repaint_changed_lines(buf, ns, old, new)
  local old_marks, new_marks = marks_by_row(old.extmarks), marks_by_row(new.extmarks)
  M.with_modifiable(buf, function()
    for row = 0, #new.lines - 1 do
      local line, marks = new.lines[row + 1], new_marks[row] or NO_MARKS
      if line ~= old.lines[row + 1] or not same_marks(old_marks[row] or NO_MARKS, marks) then
        vim.api.nvim_buf_clear_namespace(buf, ns, row, row + 1)
        vim.api.nvim_buf_set_lines(buf, row, row + 1, true, { line })
        set_marks(buf, ns, 0, marks)
      end
    end
  end)
end

---Repaints a read-only buf as new, rewriting only the lines that differ from old while buf still
---shows old; otherwise, or with another line count, it repaints the whole buffer.
---@param buf integer
---@param ns integer written by paint alone
---@param old games.Painted|nil
---@param new games.Rendered
---@return games.Painted
function M.update(buf, ns, old, new)
  if
    old
    and old.changedtick == vim.api.nvim_buf_get_changedtick(buf)
    and #old.rendered.lines == #new.lines
  then
    repaint_changed_lines(buf, ns, old.rendered, new)
  else
    paint(buf, ns, 0, -1, new)
  end
  return { rendered = new, changedtick = vim.api.nvim_buf_get_changedtick(buf) }
end

---@type games.Paint
local painter = setmetatable(M, {
  __call = function(_, ...)
    paint(...)
  end,
})
return painter
