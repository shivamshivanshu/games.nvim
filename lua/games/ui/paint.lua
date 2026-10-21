---@class games.Paint
---@field with_modifiable fun(buf: integer, fn: fun())
---@field update fun(buf: integer, ns: integer, old: games.Painted|nil, new: games.Rendered): games.Painted
---@overload fun(buf: integer, ns: integer, first: integer, last: integer, rendered: games.Rendered)

---What paint.update drew into a buffer, which still shows it while its changedtick is unchanged.
---@class games.Painted
---@field rendered games.Rendered
---@field changedtick integer

local M = {}

---@type integer[]
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
---@param marks games.Marks
---@param i integer
local function set_mark(buf, ns, first, marks, i)
  vim.api.nvim_buf_set_extmark(buf, ns, first + marks.row[i], marks.col[i], {
    end_col = marks.end_col[i],
    hl_group = marks.hl[i],
  })
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
  for i = 1, #rendered.marks.row do
    set_mark(buf, ns, first, rendered.marks, i)
  end
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

---Whether the marks a_marks lists at indices a span and colour as those b_marks lists at b do.
---@param a_marks games.Marks
---@param a integer[]
---@param b_marks games.Marks
---@param b integer[]
---@return boolean
local function same_marks(a_marks, a, b_marks, b)
  if #a ~= #b then
    return false
  end
  for k = 1, #a do
    local i, j = a[k], b[k]
    if
      a_marks.col[i] ~= b_marks.col[j]
      or a_marks.end_col[i] ~= b_marks.end_col[j]
      or not same_hl(a_marks.hl[i], b_marks.hl[j])
    then
      return false
    end
  end
  return true
end

---@param marks games.Marks
---@return table<integer, integer[]> by row, the indices of its marks in order, holding only rows that have marks
local function marks_by_row(marks)
  local by_row = {}
  for i, row in ipairs(marks.row) do
    local row_marks = by_row[row]
    if not row_marks then
      row_marks = {}
      by_row[row] = row_marks
    end
    row_marks[#row_marks + 1] = i
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
  local old_rows, new_rows = marks_by_row(old.marks), marks_by_row(new.marks)
  M.with_modifiable(buf, function()
    for row = 0, #new.lines - 1 do
      local line, marks = new.lines[row + 1], new_rows[row] or NO_MARKS
      local unchanged = line == old.lines[row + 1]
        and same_marks(old.marks, old_rows[row] or NO_MARKS, new.marks, marks)
      if not unchanged then
        vim.api.nvim_buf_clear_namespace(buf, ns, row, row + 1)
        vim.api.nvim_buf_set_lines(buf, row, row + 1, true, { line })
        for _, i in ipairs(marks) do
          set_mark(buf, ns, 0, new.marks, i)
        end
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
