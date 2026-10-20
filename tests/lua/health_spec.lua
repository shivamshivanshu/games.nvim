local Client = require("games.core.client")
local config = require("games.config")
local games = require("games")
local health = require("games.health")
local helpers = require("helpers")
local types = require("games.types")

local SECRET = "hunter2-do-not-print"
local REPORTS = { "start", "ok", "info", "warn", "error" }

---Runs the check against a stub-backed client and returns every reported line, advice included.
---@param opts games.SetupOpts
---@param conn games.test.StubConn
---@return string[]
local function report_lines(opts, conn)
  local original = { config = games.config, client = games.client, reports = {} }
  local lines = {}
  for _, report in ipairs(REPORTS) do
    original.reports[report] = vim.health[report]
    vim.health[report] = function(message, advice)
      table.insert(lines, report .. ": " .. message)
      vim.list_extend(lines, type(advice) == "table" and advice or { advice })
    end
  end
  games.config = config.resolve(opts)
  games.client = Client.new(conn --[[@as games.Conn]])
  local ok, err = pcall(health.check)
  for report, fn in pairs(original.reports) do
    vim.health[report] = fn
  end
  games.config, games.client = original.config, original.client
  assert(ok, err)
  return lines
end

---@param lines string[]
---@param needle string
---@return boolean
local function mentions(lines, needle)
  return vim.iter(lines):any(function(line)
    return line:find(needle, 1, true) ~= nil
  end)
end

local T = {}

function T.health_masks_a_string_password()
  local conn = helpers.stub_conn()
  conn.error = { code = types.ErrorCode.AUTH_FAILED, message = "bad password" }
  local lines = report_lines({ username = "alice", password = SECRET }, conn)
  assert(not mentions(lines, SECRET), vim.inspect(lines))
  assert(mentions(lines, "password: set"), vim.inspect(lines))
  assert(
    mentions(lines, "warn: disconnected from 127.0.0.1:7777: bad password"),
    vim.inspect(lines)
  )
end

function T.health_never_calls_a_password_function()
  local called = false
  local lines = report_lines({
    username = "alice",
    password = function()
      called = true
      return SECRET
    end,
  }, helpers.stub_conn())
  assert(not called and not mentions(lines, SECRET), vim.inspect(lines))
  assert(mentions(lines, "password: set (function)"), vim.inspect(lines))
end

return T
