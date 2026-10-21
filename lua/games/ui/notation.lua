local text = require("games.ui.text")

local M = {}

---The first n letters, for a board's file names.
---@param n integer at most 26
---@return string
function M.letters(n)
  return text.ALPHABET:sub(1, n)
end

---"1" to n, for a board's rank names.
---@param n integer
---@return string[]
function M.numbers(n)
  return vim.tbl_map(tostring, vim.fn.range(1, n))
end

return M
