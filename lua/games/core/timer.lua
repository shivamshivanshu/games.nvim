local MS_PER_SECOND = 1000
local US_PER_MS = 1000

local M = {}

---Wall-clock now as unix ms, the time base of every time on the wire.
---@return integer
function M.unix_ms()
  local seconds, microseconds = vim.uv.gettimeofday()
  return seconds * MS_PER_SECOND + math.floor(microseconds / US_PER_MS)
end

---Stops and closes timer unless it is already closing.
---@param timer uv.uv_timer_t|nil
function M.close(timer)
  if timer and not timer:is_closing() then
    timer:stop()
    timer:close()
  end
end

return M
