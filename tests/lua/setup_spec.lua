local games = require("games")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

---Runs fn with notify recorded; the plugin's config and client are restored after.
---@param fn fun(notified: string[])
local function with_plugin_state(fn)
  helpers.with_overrides({
    { games, "config", games.config },
    { games, "client", games.client },
  }, function()
    helpers.capture_notifications(function(notified)
      local original_client = games.client
      local ok, err = xpcall(fn, debug.traceback, notified)
      if games.client and games.client ~= original_client then
        games.client.conn:close()
      end
      assert(ok, err)
    end)
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

---@param lhs string
---@return boolean
local function mapped(lhs)
  return vim.fn.maparg(lhs, "n") ~= ""
end

function T.setup_maps_the_keys_opts_set_and_drops_those_an_earlier_setup_mapped()
  with_plugin_state(function()
    local ok, err = pcall(function()
      games.setup({ autoconnect = false, keys = { dashboard = "<leader>gg", find = "<leader>gf" } })
      assert(mapped("<leader>gg") and mapped("<leader>gf"), "keys not mapped")
      games.setup({ autoconnect = false, keys = { find = "<leader>zf" } })
      assert_eq(
        { false, false, true },
        { mapped("<leader>gg"), mapped("<leader>gf"), mapped("<leader>zf") }
      )
    end)
    games.setup({ autoconnect = false })
    assert(ok, err)
    assert(not mapped("<leader>zf"), "a setup without keys left one mapped")
  end)
end

function T.setup_leaves_a_key_the_user_remapped_since()
  with_plugin_state(function()
    local function users_own() end
    local ok, err = pcall(function()
      games.setup({ autoconnect = false, keys = { dashboard = "<leader>gg" } })
      vim.keymap.set("n", "<leader>gg", users_own)
      games.setup({ autoconnect = false })
      assert_eq(users_own, vim.fn.maparg("<leader>gg", "n", false, true).callback)
    end)
    pcall(vim.keymap.del, "n", "<leader>gg")
    assert(ok, err)
  end)
end

function T.setup_closes_the_dashboard_of_the_client_it_replaces()
  with_plugin_state(function()
    local hub = require("fixtures.hub")
    local dashboard = require("games.ui.dashboard")
    local client, conn = hub.client()
    conn.close = function() end
    games.client = client
    dashboard.open(client)
    local ok, err = pcall(function()
      assert(dashboard.is_open(), "no dashboard")
      games.setup({ autoconnect = false })
      assert(not dashboard.is_open(), "the dashboard kept following the replaced client")
    end)
    dashboard.close()
    assert(ok, err)
  end)
end

return T
