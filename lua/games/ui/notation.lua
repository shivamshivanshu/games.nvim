local text = require("games.ui.text")

local ALPHABET = "abcdefghijklmnopqrstuvwxyz"

local M = {}

---@type string[] a to z
M.LETTERS = text.chars(ALPHABET)

---The nth letter, for a board's nth file.
---@param n integer at most 26
---@return string
function M.letter(n)
  return ALPHABET:sub(n, n)
end

---The first n letters, for a board's file names.
---@param n integer at most 26
---@return string
function M.letters(n)
  return ALPHABET:sub(1, n)
end

---"1" to n, for a board's rank names.
---@param n integer
---@return string[]
function M.numbers(n)
  return vim.tbl_map(tostring, vim.fn.range(1, n))
end

---A cell named as its column's letter then its row's number, e.g. h8.
---@param cell games.Cell|games.GridPos
---@return string
function M.cell_name(cell)
  return M.letter(cell.col) .. cell.row
end

return M
