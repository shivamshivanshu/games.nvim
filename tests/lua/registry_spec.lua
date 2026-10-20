local registry = require("games.core.registry")

local T = {}

function T.missing_module_is_nil_but_load_error_raises()
  package.preload["games.games.broken"] = function()
    error("broken game module", 0)
  end
  local ok, err = pcall(registry.game_module, "broken")
  package.preload["games.games.broken"] = nil

  assert(registry.game_module("no_such_game") == nil)
  assert(registry.game_module("../chess") == nil)
  assert(not ok and err == "broken game module", vim.inspect(err))
  assert(registry.game_module("chess") == require("games.games.chess"))
end

return T
