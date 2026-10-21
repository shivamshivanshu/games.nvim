local M = {}

---@generic T
---@param item T
---@return T
local function identity(item)
  return item
end

---A set of list's items, or of key_fn of each.
---@generic T
---@param list T[]
---@param key_fn? fun(item: T): any
---@return table<any, true>
function M.set_of(list, key_fn)
  local key_of = key_fn or identity
  local set = {}
  for _, item in ipairs(list) do
    set[key_of(item)] = true
  end
  return set
end

---t's keys in sort order.
---@generic K
---@param t table<K, any>
---@return K[]
function M.sorted_keys(t)
  local keys = vim.tbl_keys(t)
  table.sort(keys)
  return keys
end

---list's items by key_fn of each; of two items with one key the later wins.
---@generic T
---@param list T[]
---@param key_fn fun(item: T): any
---@return table<any, T>
function M.index_by(list, key_fn)
  local index = {}
  for _, item in ipairs(list) do
    index[key_fn(item)] = item
  end
  return index
end

return M
