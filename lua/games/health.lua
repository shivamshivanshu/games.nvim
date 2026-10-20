local health = vim.health

local M = {}

---@param password string|fun(): string|nil
---@return string
local function password_status(password)
  if type(password) == "function" then
    return "set (function)"
  end
  return password and "set" or "missing"
end

local function check_neovim()
  local required = require("games").REQUIRED_NVIM
  local v = vim.version()
  local version = ("%d.%d.%d"):format(v.major, v.minor, v.patch)
  if vim.fn.has("nvim-" .. required) == 1 then
    health.ok("Neovim " .. version)
  else
    health.error(("Neovim %s, but games.nvim needs %s or newer"):format(version, required))
  end
end

---@param config games.Config
local function check_config(config)
  health.ok(("server: %s:%d"):format(config.server.host, config.server.port))
  health.info("username: " .. (config.username or "missing, asked for on the first :Games"))
  health.info("password: " .. password_status(config.password))
  health.info("autoconnect: " .. tostring(config.autoconnect))
end

---@param client games.Client
local function check_connection(client)
  local me = client:me()
  if me then
    health.ok("connected as " .. me)
    return
  end
  local address = client.conn:address()
  if client.conn:is_connecting() then
    health.info("connecting to " .. address)
    return
  end
  local last_error = client.conn:last_error()
  if not last_error then
    health.info(("not connected to %s; :Games connects"):format(address))
    return
  end
  health.warn(
    ("disconnected from %s: %s (%s)"):format(address, last_error.message, last_error.code)
  )
end

---@param glyphs games.Glyphs
local function check_glyphs(glyphs)
  if glyphs == "ascii" then
    health.ok('glyphs = "ascii"')
    return
  end
  health.info(('glyphs = "%s"; if pieces render badly, set glyphs = "ascii"'):format(glyphs))
end

---Run by `:checkhealth games`.
function M.check()
  health.start("games.nvim")
  check_neovim()
  local games = require("games")
  if not (games.config and games.client) then
    health.error("setup() was not called", { 'call require("games").setup({}) in your config' })
    return
  end
  health.ok("setup() was called")

  health.start("games.nvim: config")
  check_config(games.config)
  health.start("games.nvim: connection")
  check_connection(games.client)
  health.start("games.nvim: glyphs")
  check_glyphs(games.config.glyphs)
end

return M
