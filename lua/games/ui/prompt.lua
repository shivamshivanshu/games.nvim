local YES = 1
local NO = 2

local M = {}

---Asks question with Yes and No, No the default.
---@param question string
---@return boolean
function M.confirm(question)
  return vim.fn.confirm(question, "&Yes\n&No", NO) == YES
end

return M
