local games = require("games")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

---Runs fn with notify recorded; the plugin's config and client are restored after.
---@param fn fun(notified: string[])
local function with_plugin_state(fn)
  local notified = {}
  helpers.with_overrides({
    { games, "config", games.config },
    { games, "client", games.client },
    {
      vim,
      "notify",
      function(message)
        table.insert(notified, message)
      end,
    },
  }, function()
    local original_client = games.client
    local ok, err = xpcall(fn, debug.traceback, notified)
    if games.client and games.client ~= original_client then
      games.client.conn:close()
    end
    assert(ok, err)
  end)
end

local T = {}

function T.setup_without_credentials_skips_autoconnect()
  with_plugin_state(function()
    local scheduled = {}
    local function defer(fn)
      table.insert(scheduled, fn)
    end
    helpers.with_overrides({ { vim, "schedule", defer } }, function()
      games.setup({ server = { port = 1 } })
    end)
    local connects = 0
    assert(games.client).conn.connect = function()
      connects = connects + 1
    end
    vim.api.nvim_exec_autocmds("VimEnter", { group = "games.autoconnect" })
    for _, fn in ipairs(scheduled) do
      fn()
    end
    helpers.assert_eq(0, connects)
  end)
end

function T.setup_defines_the_palette_before_any_board_opens()
  with_plugin_state(function(notified)
    vim.api.nvim_set_hl(0, "GamesSquareLight", {})
    helpers.with_overrides({ { vim.o, "termguicolors", false } }, function()
      games.setup({ autoconnect = false, palette = "vivid" })
    end)
    assert(vim.api.nvim_get_hl(0, { name = "GamesSquareLight" }).bg, "GamesSquareLight unset")
    assert_eq(1, #vim.api.nvim_get_autocmds({ group = "games.theme", event = "ColorScheme" }))
    assert(
      vim.iter(notified):any(function(message)
        return message:find("termguicolors", 1, true) ~= nil
      end),
      vim.inspect(notified)
    )
  end)
end

function T.setup_on_old_neovim_notifies_and_returns()
  with_plugin_state(function(notified)
    local function unsupported()
      return false
    end
    helpers.with_overrides({ { games, "neovim_supported", unsupported } }, function()
      games.setup({ username = 42 })
    end)
    helpers.assert_eq({ "games: needs Neovim 0.11 or newer" }, notified)
  end)
end

return T
