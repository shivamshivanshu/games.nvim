local M = {}

---Stops and closes timer unless it is already closing.
---@param timer uv.uv_timer_t|nil
function M.close(timer)
  if timer and not timer:is_closing() then
    timer:stop()
    timer:close()
  end
end

return M
