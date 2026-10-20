local Conn = require("games.core.conn")

local M = {}

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
local SERVER_BIN = REPO_ROOT .. "/bin/games-server"
local USERS = { "alice", "bob" }
M.PASSWORD = "pw"
M.WAIT_MS = 2000

---@class games.test.Server
---@field port integer
---@field pause fun()
---@field stop fun()

---@param db string
---@param username string
local function add_user(db, username)
  local result = vim
    .system(
      { SERVER_BIN, "adduser", "--db", db, "--password-stdin", username },
      { stdin = M.PASSWORD .. "\n" }
    )
    :wait()
  assert(result.code == 0, ("adduser %s failed: %s"):format(username, result.stderr))
end

---@param serve_args? string[]
---@return games.test.Server
function M.start_server(serve_args)
  local dir = vim.fn.tempname()
  vim.fn.mkdir(dir, "p")
  local db = dir .. "/games.db"
  for _, username in ipairs(USERS) do
    add_user(db, username)
  end

  local port
  local cmd =
    vim.list_extend({ SERVER_BIN, "serve", "--addr", "127.0.0.1:0", "--db", db }, serve_args or {})
  local process = vim.system(cmd, {
    stderr = false,
    stdout = function(_, data)
      port = port or (data and tonumber(data:match("^listening on 127%.0%.0%.1:(%d+)")))
    end,
  })

  local stopped = false
  local function stop()
    if stopped then
      return
    end
    stopped = true
    process:kill("sigkill")
    process:wait()
    vim.fn.delete(dir, "rf")
  end

  if not vim.wait(M.WAIT_MS, function()
    return port ~= nil
  end) then
    stop()
    error("games-server did not report its port")
  end

  return {
    port = port,
    pause = function()
      process:kill("sigstop")
    end,
    stop = stop,
  }
end

---@class games.test.Env
---@field server games.test.Server
---@field new_conn fun(username: string, password?: string): games.Conn

---Runs fn against a fresh server and closes every conn it made and the server, even on failure.
---@param serve_args string[]|nil
---@param fn fun(env: games.test.Env)
function M.with_server(serve_args, fn)
  local server = M.start_server(serve_args)
  local conns = {}
  local env = {
    server = server,
    new_conn = function(username, password)
      local conn = Conn.new({
        host = "127.0.0.1",
        port = server.port,
        username = username,
        password = password or M.PASSWORD,
      })
      table.insert(conns, conn)
      return conn
    end,
  }
  local ok, err = xpcall(fn, debug.traceback, env)
  for _, conn in ipairs(conns) do
    conn:close()
  end
  server.stop()
  if not ok then
    error(err, 0)
  end
end

---@class games.test.EventLog
---@field connected games.LoginResult[]
---@field disconnected games.ErrorBody[]

---@param conn games.Conn
---@return games.test.EventLog
function M.record_lifecycle(conn)
  local log = { connected = {}, disconnected = {} }
  conn:on("connected", function(result)
    table.insert(log.connected, result)
  end)
  conn:on("disconnected", function(err)
    table.insert(log.disconnected, err)
  end)
  return log
end

---@param predicate fun(): boolean
---@param what string
function M.wait_for(predicate, what)
  assert(vim.wait(M.WAIT_MS, predicate), "timed out waiting for " .. what)
end

return M
