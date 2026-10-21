local helpers = require("helpers")
local timer = require("games.core.timer")

local assert_eq = helpers.assert_eq

---@return integer
local function timers_alive()
  local count = 0
  vim.uv.walk(function(handle)
    if handle:get_type() == "timer" and not handle:is_closing() then
      count = count + 1
    end
  end)
  return count
end

local T = {}

function T.a_ticker_ticks_only_while_it_is_needed_and_holds_a_timer_only_then()
  local ticks = 0
  local before = timers_alive()
  local ticker = timer.ticker(5, function()
    ticks = ticks + 1
  end)
  assert_eq(before, timers_alive())

  ticker.sync(true)
  ticker.sync(true)
  assert_eq(before + 1, timers_alive())
  helpers.wait_for(function()
    return ticks >= 2
  end, "two ticks")
  ticker.sync(false)
  local stopped_at = ticks
  vim.wait(30)

  assert_eq(stopped_at, ticks)
  assert_eq(before, timers_alive())
end

function T.coalesced_calls_run_once_on_the_next_tick()
  local runs = 0
  local run = timer.coalesced(function()
    runs = runs + 1
  end)
  run()
  run()
  assert_eq(0, runs)
  helpers.wait_for(function()
    return runs > 0
  end, "the coalesced run")
  run()
  helpers.wait_for(function()
    return runs > 1
  end, "a later run")

  assert_eq(2, runs)
end

return T
