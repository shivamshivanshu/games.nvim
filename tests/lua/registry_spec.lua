local describe = require("games.core.describe")
local registry = require("games.core.registry")

local T = {}

function T.missing_is_retried_load_error_is_cached_success_is_returned()
  assert(registry.game_module("late_game") == nil)
  local late = { layout = function() end }
  package.preload["games.games.late_game"] = function()
    return late
  end
  assert(registry.game_module("late_game") == late, "a missing module is looked up again")

  local runs = 0
  package.preload["games.games.broken"] = function()
    runs = runs + 1
    error("broken game module", 0)
  end
  local first_ok, first_err = pcall(registry.game_module, "broken")
  local second_ok, second_err = pcall(registry.game_module, "broken")
  local described = describe.events({
    game = "broken",
    players = { { user = "bob", seat = "north" } },
    events = { { kind = "played", by = "north" } },
  })
  package.preload["games.games.broken"] = nil
  package.preload["games.games.late_game"] = nil

  assert(not first_ok and not second_ok, "a load error raises on every lookup")
  assert(first_err == "broken game module" and second_err == first_err, vim.inspect(second_err))
  assert(runs == 1, "the broken file ran " .. runs .. " times")
  assert(described == "bob: played", described)
  assert(registry.game_module("../chess") == nil and registry.game_module("chess.x") == nil)
  assert(registry.game_module("chess") == require("games.games.chess"))
end

return T
