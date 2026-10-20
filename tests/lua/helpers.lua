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

---@alias games.test.Override {[1]: table, [2]: string, [3]: any} target, key, value

---Sets each target[key] = value while fn runs and restores the originals, even when fn fails.
---@param overrides games.test.Override[]
---@param fn fun()
function M.with_overrides(overrides, fn)
  local originals = {}
  for index, override in ipairs(overrides) do
    originals[index] = override[1][override[2]]
    override[1][override[2]] = override[3]
  end
  local ok, err = xpcall(fn, debug.traceback)
  for index = #overrides, 1, -1 do
    local override = overrides[index]
    override[1][override[2]] = originals[index]
  end
  if not ok then
    error(err, 0)
  end
end

---A chess board's view options, drawing from the default asset pack.
---@param glyphs games.Glyphs
---@param flipped? boolean
---@return games.ViewOptions
function M.chess_view(glyphs, flipped)
  local assets = require("games.ui.assets")
  local spec = { keys = require("games.games.chess").asset_keys, glyph_width = 1 }
  local pack = assets.load("chess", assets.DEFAULT_PACK, spec)
  return { glyphs = glyphs, flipped = flipped == true, art = assets.art_set(pack, glyphs) }
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

---Runs fn with a fresh temp dir path, not yet created, and removes the dir afterwards.
---@param fn fun(dir: string)
function M.with_temp_dir(fn)
  local dir = vim.fn.tempname()
  local ok, err = xpcall(fn, debug.traceback, dir)
  vim.fn.delete(dir, "rf")
  if not ok then
    error(err, 0)
  end
end

---@param path string
---@return string|nil nil when the file does not exist
function M.read_file(path)
  local file = io.open(path, "rb")
  if not file then
    return nil
  end
  local data = file:read("*a")
  file:close()
  return data
end

---A copy of update finished with summary, carrying summary's digest as the server would.
---@param update games.GameUpdate
---@param summary string
---@return games.GameUpdate
function M.seal(update, summary)
  return vim.tbl_extend("force", update, { summary = summary, digest = vim.fn.sha256(summary) })
end

---@class games.test.StubConn
---@field handlers table<string, fun(body: table)[]>
---@field requests {type: string, body: table|nil, cb: fun(err: games.ErrorBody|nil, result: table|nil), frame: string}[] frame as Conn encodes it
---@field credentials games.Credentials
---@field connects integer how often connect() was called
---@field connecting boolean
---@field error? games.ErrorBody what last_error() returns

---Records requests instead of sending them; push() delivers an event synchronously.
---@param credentials? games.Credentials
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
  function conn:fill_credentials(entered)
    self.credentials = vim.tbl_extend("keep", self.credentials, entered)
  end
  function conn:missing_credentials()
    return vim.tbl_filter(function(field)
      return self.credentials[field] == nil
    end, { "username", "password" })
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
    local _, frame = Conn.encode_request(#self.requests + 1, message_type, body)
    table.insert(self.requests, { type = message_type, body = body, cb = cb, frame = frame })
  end
  function conn:push(event, body)
    for _, fn in ipairs(self.handlers[event]) do
      fn(body)
    end
  end
  return conn
end

return M
