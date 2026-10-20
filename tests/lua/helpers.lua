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
---@field restart fun() kills the server and starts a new one on the same port and database
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

---@param db string
---@param addr string
---@param serve_args string[]
---@return vim.SystemObj process
---@return integer|nil port nil if the server did not report one in time
local function spawn(db, addr, serve_args)
  local port
  local cmd = vim.list_extend({ SERVER_BIN, "serve", "--addr", addr, "--db", db }, serve_args)
  local process = vim.system(cmd, {
    stderr = false,
    stdout = function(_, data)
      port = port or (data and tonumber(data:match("^listening on 127%.0%.0%.1:(%d+)")))
    end,
  })
  vim.wait(M.WAIT_MS, function()
    return port ~= nil
  end)
  return process, port
end

---@param process vim.SystemObj
local function kill(process)
  process:kill("sigkill")
  process:wait()
end

---@param serve_args? string[]
---@return games.test.Server
function M.start_server(serve_args)
  serve_args = serve_args or {}
  local dir = vim.fn.tempname()
  vim.fn.mkdir(dir, "p")
  local db = dir .. "/games.db"
  local added, add_err = pcall(function()
    for _, username in ipairs(USERS) do
      add_user(db, username)
    end
  end)
  if not added then
    vim.fn.delete(dir, "rf")
    error(add_err, 0)
  end

  local process, port = spawn(db, "127.0.0.1:0", serve_args)
  local stopped = false
  local function stop()
    if stopped then
      return
    end
    stopped = true
    kill(process)
    vim.fn.delete(dir, "rf")
  end
  if not port then
    stop()
    error("games-server did not report its port")
  end

  return {
    port = port,
    pause = function()
      process:kill("sigstop")
    end,
    restart = function()
      kill(process)
      local restarted_port
      process, restarted_port = spawn(db, "127.0.0.1:" .. port, serve_args)
      assert(restarted_port == port, "games-server did not restart on port " .. port)
    end,
    stop = stop,
  }
end

---@class games.test.Env
---@field server games.test.Server
---@field new_conn fun(username: string, password?: string|fun(): string?): games.Conn

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
---@field disconnected games.Disconnect[]

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

---@param expected any
---@param actual any
function M.assert_eq(expected, actual)
  assert(vim.deep_equal(expected, actual), vim.inspect({ expected = expected, actual = actual }))
end

---@param predicate fun(): boolean
---@param what string
function M.wait_for(predicate, what)
  assert(vim.wait(M.WAIT_MS, predicate), "timed out waiting for " .. what)
end

---@class games.test.StubCredentials
---@field username? string
---@field password? string|fun(): string

---@class games.test.StubConn
---@field handlers table<string, fun(body: table)[]>
---@field requests {type: string, body: table|nil, cb: fun(err: games.ErrorBody|nil, result: table|nil)}[]
---@field credentials games.test.StubCredentials
---@field connects integer how often connect() was called
---@field connecting boolean
---@field error? games.ErrorBody what last_error() returns

---Records requests instead of sending them; push() delivers an event synchronously.
---@param credentials? games.test.StubCredentials
---@return games.test.StubConn
function M.stub_conn(credentials)
  local conn = {
    handlers = vim.defaulttable(),
    requests = {},
    credentials = credentials or {},
    connects = 0,
    connecting = false,
  }
  function conn:connect()
    self.connects = self.connects + 1
  end
  function conn:set_credentials(username, password)
    self.credentials = { username = username, password = password }
  end
  function conn:has_credentials()
    return self.credentials.username ~= nil and self.credentials.password ~= nil
  end
  function conn:username()
    return self.credentials.username
  end
  function conn:address()
    return "127.0.0.1:7777"
  end
  function conn:is_connecting()
    return self.connecting
  end
  function conn:last_error()
    return self.error
  end
  function conn:on(event, fn)
    table.insert(self.handlers[event], fn)
  end
  function conn:request(message_type, body, cb)
    table.insert(self.requests, { type = message_type, body = body, cb = cb })
  end
  function conn:push(event, body)
    for _, fn in ipairs(self.handlers[event]) do
      fn(body)
    end
  end
  return conn
end

return M
