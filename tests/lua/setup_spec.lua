local games = require("games")
local helpers = require("helpers")

---Runs fn with notify recorded; the plugin's config and client are restored after.
---@param fn fun(notified: string[])
local function with_plugin_state(fn)
  local original = { config = games.config, client = games.client, notify = vim.notify }
  local notified = {}
  vim.notify = function(message)
    table.insert(notified, message)
  end
  local ok, err = xpcall(fn, debug.traceback, notified)
  if games.client and games.client ~= original.client then
    games.client.conn:close()
  end
  games.config, games.client, vim.notify = original.config, original.client, original.notify
  assert(ok, err)
end

local T = {}

function T.setup_without_credentials_skips_autoconnect()
  with_plugin_state(function()
    games.setup({ server = { port = 1 } })
    local conn = assert(games.client).conn
    local connects = 0
    conn.connect = function()
      connects = connects + 1
    end
    vim.api.nvim_exec_autocmds("VimEnter", { group = "games.autoconnect" })
    vim.wait(20)
    helpers.assert_eq(0, connects)
  end)
end

return T
