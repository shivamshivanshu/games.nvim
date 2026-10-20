local Client = require("games.core.client")
local Conn = require("games.core.conn")
local config = require("games.config")
local notify = require("games.core.notify")

local AUTOCONNECT_GROUP = "games.autoconnect"

local M = {}

---@type games.Config|nil
M.config = nil

---@type games.Client|nil
M.client = nil

local function connect_current()
  if M.client then
    M.client.conn:connect()
  end
end

---Validates opts, replaces any previous connection, and connects after startup when autoconnect
---is set and both credentials are known.
---@param opts games.SetupOpts
function M.setup(opts)
  M.config = config.resolve(opts)
  if M.client then
    M.client.conn:close()
  end
  M.client = Client.new(Conn.new({
    host = M.config.server.host,
    port = M.config.server.port,
    username = M.config.username,
    password = M.config.password,
  }))
  notify.attach(M.client)

  local group = vim.api.nvim_create_augroup(AUTOCONNECT_GROUP, { clear = true })
  if not (M.config.autoconnect and M.client.conn:has_credentials()) then
    return
  end
  if vim.v.vim_did_enter == 1 then
    vim.schedule(connect_current)
    return
  end
  vim.api.nvim_create_autocmd("VimEnter", {
    group = group,
    once = true,
    callback = function()
      vim.schedule(connect_current)
    end,
  })
end

---Options for every board this plugin opens, taken from the user's config.
---@return games.OpenOptions
function M.open_options()
  return { glyphs = M.config and M.config.glyphs or config.DEFAULT_GLYPHS }
end

return M
