local M = {}

---@alias games.Glyphs "unicode"|"ascii"

---@class games.ServerAddress
---@field host string
---@field port integer

---@class games.Config
---@field server games.ServerAddress
---@field username? string asked for on the first :Games when nil
---@field password? string|fun(): string resolved at each connect, never during setup; asked for when nil
---@field autoconnect boolean
---@field glyphs games.Glyphs

---@class games.SetupOpts
---@field server? {host?: string, port?: integer}
---@field username? string
---@field password? string|fun(): string
---@field autoconnect? boolean
---@field glyphs? games.Glyphs

local GLYPHS = { unicode = true, ascii = true }

---@type games.Glyphs
M.DEFAULT_GLYPHS = "unicode"

local DEFAULTS = {
  server = { host = "127.0.0.1", port = 7777 },
  autoconnect = true,
  glyphs = M.DEFAULT_GLYPHS,
}

---@param port any
---@return boolean
local function is_port(port)
  return type(port) == "number" and port == math.floor(port) and port > 0 and port < 65536
end

---Merges opts over the defaults and validates the result.
---@param opts games.SetupOpts
---@return games.Config
function M.resolve(opts)
  vim.validate("opts", opts, "table")
  local config = vim.tbl_deep_extend("force", DEFAULTS, opts)
  vim.validate("server.host", config.server.host, "string")
  vim.validate("server.port", config.server.port, is_port, "port number 1-65535")
  vim.validate("username", config.username, "string", true)
  vim.validate("password", config.password, { "string", "function" }, true)
  vim.validate("autoconnect", config.autoconnect, "boolean")
  vim.validate("glyphs", config.glyphs, function(glyphs)
    return GLYPHS[glyphs] ~= nil
  end, '"unicode" or "ascii"')
  return config
end

return M
