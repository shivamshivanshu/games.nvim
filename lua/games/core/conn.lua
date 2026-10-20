local types = require("games.types")

local MessageType = types.MessageType
local ErrorCode = types.ErrorCode

local CONNECT_TIMEOUT_MS = 10000
local IDLE_HEARTBEATS = 3
local RECONNECT_MIN_MS = 1000
local RECONNECT_MAX_MS = 30000
local RECONNECT_JITTER = 0.2
local JSON_DECODE_OPTS = { luanil = { object = true, array = true } }
local FATAL_LOGIN_ERRORS = { [ErrorCode.AUTH_FAILED] = true, [ErrorCode.VERSION_MISMATCH] = true }

---@alias games.ConnEvent "connected"|"disconnected"|"challenge.changed"|"game.update"

---@class games.ConnOpts
---@field host string
---@field port integer
---@field username string
---@field password string|fun(): string

---One TCP connection try; every libuv callback checks it is still current before acting.
---@class (private) games.ConnAttempt
---@field password string
---@field buffer string
---@field socket? uv.uv_tcp_t
---@field opened boolean
---@field authenticated boolean
---@field idle_ms integer
---@field watchdog uv.uv_timer_t
---@field heartbeat? uv.uv_timer_t

---@class games.Conn
---@field private opts games.ConnOpts
---@field private attempt? games.ConnAttempt
---@field private pending table<integer, games.ReplyCallback>
---@field private next_id integer
---@field private handlers table<string, fun(payload: any)[]>
---@field private auto_reconnect boolean
---@field private backoff_ms integer
---@field private reconnect_timer? uv.uv_timer_t
local Conn = {}
Conn.__index = Conn

---@param message string
---@return games.ErrorBody
local function disconnected(message)
  return { code = ErrorCode.DISCONNECTED, message = message }
end

---@param cb games.ReplyCallback
---@param err games.ErrorBody|nil
---@param result table|nil
local function deliver(cb, err, result)
  vim.schedule(function()
    cb(err, result)
  end)
end

local function ignore_reply() end

---@param password string|fun(): string
---@return string|nil password
---@return string|nil problem why there is no usable password
local function resolve_password(password)
  local value = password
  if type(password) == "function" then
    local ok, result = pcall(password)
    if not ok then
      return nil, "password function failed: " .. tostring(result)
    end
    value = result
  end
  if type(value) ~= "string" or value == "" then
    return nil,
      type(password) == "function" and "password function returned no password"
        or "password is empty"
  end
  return value
end

---@param timer uv.uv_timer_t|nil
local function close_timer(timer)
  if timer and not timer:is_closing() then
    timer:stop()
    timer:close()
  end
end

---@param opts games.ConnOpts
---@return games.Conn
function Conn.new(opts)
  return setmetatable({
    opts = opts,
    pending = {},
    next_id = 1,
    handlers = {},
    auto_reconnect = false,
    backoff_ms = RECONNECT_MIN_MS,
  }, Conn)
end

---Splits newline-delimited frames out of the unterminated tail of earlier reads plus a new chunk.
---@param buffer string
---@param chunk string
---@return string[] lines complete frames, without the newline
---@return string rest the unterminated tail to keep for the next read
function Conn.split_frames(buffer, chunk)
  local lines = vim.split(buffer .. chunk, "\n", { plain = true })
  local rest = table.remove(lines)
  return lines, rest
end

---Empty Lua tables encode as `[]`; wire objects (request bodies, options) must encode as `{}`.
---@param value table|nil
---@return table
function Conn.as_json_object(value)
  if value == nil or vim.tbl_isempty(value) then
    return vim.empty_dict()
  end
  return value
end

---Subscribes fn to a push type or a lifecycle event; fn runs via vim.schedule.
---"connected" gets the games.LoginResult, "disconnected" the games.ErrorBody that ended the session.
---@param event games.ConnEvent
---@param fn fun(payload: any)
function Conn:on(event, fn)
  self.handlers[event] = self.handlers[event] or {}
  table.insert(self.handlers[event], fn)
end

---Connects and logs in, reconnecting with backoff until close() or a fatal login error.
function Conn:connect()
  self.auto_reconnect = true
  self.backoff_ms = RECONNECT_MIN_MS
  if self.reconnect_timer then
    self.reconnect_timer:stop()
  end
  if not self.attempt then
    self:open()
  end
end

---Sends a request once logged in; otherwise cb gets a "disconnected" error.
---@param message_type games.MessageType
---@param body table|nil
---@param cb games.ReplyCallback
function Conn:request(message_type, body, cb)
  local attempt = self.attempt
  if not (attempt and attempt.authenticated) then
    deliver(cb, disconnected("not connected"), nil)
    return
  end
  self:send_request(attempt, message_type, body, cb)
end

---Stops reconnecting and drops the connection.
function Conn:close()
  self.auto_reconnect = false
  close_timer(self.reconnect_timer)
  self.reconnect_timer = nil
  if self.attempt then
    self:teardown(self.attempt, disconnected("closed"))
  end
end

---@private
---@param event string
---@param payload any
function Conn:emit(event, payload)
  for _, fn in ipairs(self.handlers[event] or {}) do
    vim.schedule(function()
      fn(payload)
    end)
  end
end

---@private
function Conn:open()
  local password, problem = resolve_password(self.opts.password)
  if not password then
    self.auto_reconnect = false
    self:emit("disconnected", { code = ErrorCode.AUTH_FAILED, message = problem })
    return
  end
  ---@type games.ConnAttempt
  local attempt = {
    password = password,
    buffer = "",
    opened = false,
    authenticated = false,
    idle_ms = CONNECT_TIMEOUT_MS,
    watchdog = assert(vim.uv.new_timer()),
  }
  self.attempt = attempt
  self:arm_watchdog(attempt)

  local host, port = self.opts.host, self.opts.port
  vim.uv.getaddrinfo(host, tostring(port), { socktype = "stream" }, function(err, addresses)
    if self.attempt ~= attempt then
      return
    end
    if err or not addresses then
      self:teardown(attempt, disconnected(("cannot resolve %s: %s"):format(host, err)))
      return
    end
    self:try_address(attempt, addresses, 1)
  end)
end

---@private
---@param attempt games.ConnAttempt
---@param addresses uv.address[]
---@param index integer
function Conn:try_address(attempt, addresses, index)
  local address = addresses[index]
  if not address then
    self:teardown(
      attempt,
      disconnected(("cannot connect to %s:%d"):format(self.opts.host, self.opts.port))
    )
    return
  end
  local socket = assert(vim.uv.new_tcp())
  attempt.socket = socket
  local function try_next_address()
    socket:close()
    attempt.socket = nil
    self:try_address(attempt, addresses, index + 1)
  end
  local _, connect_err = socket:connect(address.addr, address.port, function(err)
    if self.attempt ~= attempt then
      return
    end
    if err then
      try_next_address()
      return
    end
    attempt.opened = true
    socket:read_start(function(read_err, chunk)
      self:receive(attempt, read_err, chunk)
    end)
    self:login(attempt)
  end)
  if connect_err then
    try_next_address()
  end
end

---@private
---@param attempt games.ConnAttempt
function Conn:login(attempt)
  ---@type games.LoginRequest
  local body =
    { v = types.PROTOCOL_VERSION, username = self.opts.username, password = attempt.password }
  self:send_request(attempt, MessageType.AUTH_LOGIN, body, function(err, result)
    if self.attempt ~= attempt then
      return
    end
    if err then
      if FATAL_LOGIN_ERRORS[err.code] then
        self.auto_reconnect = false
      end
      self:teardown(attempt, err)
      return
    end
    ---@cast result games.LoginResult
    attempt.authenticated = true
    attempt.idle_ms = IDLE_HEARTBEATS * result.heartbeat_ms
    self.backoff_ms = RECONNECT_MIN_MS
    self:arm_watchdog(attempt)
    self:start_heartbeat(attempt, result.heartbeat_ms)
    self:emit("connected", result)
  end)
end

---@private
---@param attempt games.ConnAttempt
---@param interval_ms integer
function Conn:start_heartbeat(attempt, interval_ms)
  attempt.heartbeat = assert(vim.uv.new_timer())
  attempt.heartbeat:start(interval_ms, interval_ms, function()
    if self.attempt == attempt then
      self:send_request(attempt, MessageType.PING, nil, ignore_reply)
    end
  end)
end

---@private
---@param attempt games.ConnAttempt
function Conn:arm_watchdog(attempt)
  attempt.watchdog:start(attempt.idle_ms, 0, function()
    self:teardown(attempt, disconnected("server stopped responding"))
  end)
end

---@private
---@param attempt games.ConnAttempt
---@param message_type games.MessageType
---@param body table|nil
---@param cb games.ReplyCallback
function Conn:send_request(attempt, message_type, body, cb)
  local id = self.next_id
  self.next_id = id + 1
  ---@type games.Envelope
  local envelope = { id = id, type = message_type, body = Conn.as_json_object(body) }
  local encoded, frame = pcall(vim.json.encode, envelope)
  if not encoded then
    local message = ("cannot encode %s request: %s"):format(message_type, frame)
    deliver(cb, { code = ErrorCode.BAD_REQUEST, message = message }, nil)
    return
  end

  self.pending[id] = cb
  local socket = assert(attempt.socket)
  local _, write_err = socket:write(frame .. "\n", function(err)
    if err then
      self:teardown(attempt, disconnected("write failed: " .. err))
    end
  end)
  if write_err then
    self:teardown(attempt, disconnected("write failed: " .. write_err))
  end
end

---@private
---@param attempt games.ConnAttempt
---@param err string|nil
---@param chunk string|nil
function Conn:receive(attempt, err, chunk)
  if self.attempt ~= attempt then
    return
  end
  if err or not chunk then
    self:teardown(attempt, disconnected(err or "server closed the connection"))
    return
  end
  self:arm_watchdog(attempt)
  local lines
  lines, attempt.buffer = Conn.split_frames(attempt.buffer, chunk)
  for _, line in ipairs(lines) do
    if self.attempt ~= attempt then
      return
    end
    if line ~= "" then
      self:dispatch(attempt, line)
    end
  end
end

---@private
---@param attempt games.ConnAttempt
---@param line string
function Conn:dispatch(attempt, line)
  local ok, envelope = pcall(vim.json.decode, line, JSON_DECODE_OPTS)
  if not ok or type(envelope) ~= "table" then
    self:teardown(attempt, disconnected("malformed frame from server"))
    return
  end
  ---@cast envelope games.Envelope
  if envelope.id == nil then
    self:emit(envelope.type, envelope.body)
    return
  end
  local cb = self.pending[envelope.id]
  self.pending[envelope.id] = nil
  if not cb then
    return
  end
  if envelope.type == MessageType.ERROR then
    deliver(cb, envelope.body --[[@as games.ErrorBody]], nil)
  else
    deliver(cb, nil, envelope.body)
  end
end

---@private
---@param attempt games.ConnAttempt
---@param reason games.ErrorBody
function Conn:teardown(attempt, reason)
  if self.attempt ~= attempt then
    return
  end
  self.attempt = nil
  close_timer(attempt.watchdog)
  close_timer(attempt.heartbeat)
  if attempt.socket and not attempt.socket:is_closing() then
    attempt.socket:close()
  end

  local pending = self.pending
  self.pending = {}
  for _, cb in pairs(pending) do
    deliver(cb, disconnected("connection lost"), nil)
  end
  if attempt.opened then
    self:emit("disconnected", reason)
  end
  if self.auto_reconnect then
    self:schedule_reconnect()
  end
end

---@private
function Conn:schedule_reconnect()
  local delay_ms = math.floor(self.backoff_ms * (1 - RECONNECT_JITTER * math.random()))
  self.backoff_ms = math.min(2 * self.backoff_ms, RECONNECT_MAX_MS)
  self.reconnect_timer = self.reconnect_timer or assert(vim.uv.new_timer())
  self.reconnect_timer:start(
    delay_ms,
    0,
    vim.schedule_wrap(function()
      if self.auto_reconnect and not self.attempt then
        self:open()
      end
    end)
  )
end

return Conn
