local DECODE_OPTS = { luanil = { object = true, array = true } }

local M = {}

---Decodes text with JSON null as nil, in objects and arrays alike; raises on malformed text.
---@param text string
---@return any
function M.decode(text)
  return vim.json.decode(text, DECODE_OPTS)
end

---Empty Lua tables encode as `[]`, so a wire object is marked to encode as `{}` even when empty.
---@generic T: table
---@param fields T
---@return T
function M.object(fields)
  if vim.tbl_isempty(fields) then
    return vim.empty_dict()
  end
  return fields
end

return M
