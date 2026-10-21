local Signal = require("games.core.signal")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local T = {}

function T.emit_calls_every_handler_past_one_that_raises_and_the_list_it_started_with()
  local signal = Signal.new()
  local calls = {}
  local unsubscribe_b
  signal.connect(function(x)
    calls[#calls + 1] = "a" .. x
    unsubscribe_b()
    signal.connect(function()
      calls[#calls + 1] = "late"
    end)
  end)
  unsubscribe_b = signal.connect(function(x)
    calls[#calls + 1] = "b" .. x
    error("boom", 0)
  end)
  local errors = signal.emit(1)
  assert_eq({ "a1", "b1" }, calls)
  assert_eq({ "boom" }, errors)
  assert_eq(2, #signal.handlers())
end

return T
