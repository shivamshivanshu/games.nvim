local US_PER_MS = 1000

---@class games.Ticker
---@field sync fun(needed: boolean) starts the ticks when needed, else stops them and frees the timer
---@field close fun()

local M = {}

M.MS_PER_SECOND = 1000
---How often a running clock is redrawn.
M.CLOCK_TICK_MS = 1000

---Wall-clock now as unix ms, the time base of every time on the wire.
---@return integer
function M.unix_ms()
  local seconds, microseconds = vim.uv.gettimeofday()
  return seconds * M.MS_PER_SECOND + math.floor(microseconds / US_PER_MS)
end

---Stops and closes timer unless it is already closing.
---@param timer uv.uv_timer_t|nil
function M.close(timer)
  if timer and not timer:is_closing() then
    timer:stop()
    timer:close()
  end
end

---Runs fn on the main loop every ms while it is synced as needed; the timer exists only then.
---@param ms integer
---@param fn fun()
---@return games.Ticker
function M.ticker(ms, fn)
  ---@type uv.uv_timer_t|nil
  local handle
  local function sync(needed)
    if needed and not handle then
      handle = assert(vim.uv.new_timer())
      handle:start(ms, ms, vim.schedule_wrap(fn))
    elseif not needed and handle then
      M.close(handle)
      handle = nil
    end
  end
  return {
    sync = sync,
    close = function()
      sync(false)
    end,
  }
end

---fn deferred to the next main-loop turn; calls made before it runs coalesce into that one run.
---@param fn fun()
---@return fun()
function M.coalesced(fn)
  local pending = false
  return function()
    if pending then
      return
    end
    pending = true
    vim.schedule(function()
      pending = false
      fn()
    end)
  end
end

return M
