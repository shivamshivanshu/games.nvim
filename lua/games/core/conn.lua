local Signal = require("games.core.signal")
local json = require("games.core.json")
local timer = require("games.core.timer")
local types = require("games.types")

local MessageType = types.MessageType
local ErrorCode = types.ErrorCode

local CONNECT_TIMEOUT_MS = 10000
local IDLE_HEARTBEATS = 3
local RECONNECT_MIN_MS = 1000
local RECONNECT_MAX_MS = 30000
local RECONNECT_JITTER = 0.2
---Login errors after which the prompted credentials name no account, so they are asked again.
local CREDENTIALS_REJECTED = {
  [ErrorCode.AUTH_FAILED] = true,
  [ErrorCode.SIGNUP_DISABLED] = true,
  [ErrorCode.PASSPHRASE_REJECTED] = true,
  [ErrorCode.USER_EXISTS] = true,
}
local FATAL_LOGIN_ERRORS =
  vim.tbl_extend("error", CREDENTIALS_REJECTED, { [ErrorCode.VERSION_MISMATCH] = true })
---@type games.CredentialField[]
local CREDENTIAL_FIELDS = { "username", "password" }

---@alias games.ConnEvent "connected"|"disconnected"|"challenge.changed"|"game.update"

---@class games.Disconnect: games.ErrorBody
---@field retrying boolean false once the conn has stopped reconnecting

---@alias games.CredentialField "username"|"password"

---@alias games.CredentialSource "config"|"prompt"

---@class games.Credentials
---@field username? string
---@field password? string|fun(): string

---@class games.ConnOpts: games.Credentials
---@field host string
---@field port integer

---How far one connection try has got, in order.
---@enum games.ConnPhase
local Phase = {
  CONNECTING = "connecting",
  OPEN = "open",
  AUTHENTICATED = "authenticated",
}

---One TCP connection try; every libuv callback checks it is still current before acting.
---@class (private) games.ConnAttempt
---@field buffer string
---@field socket? uv.uv_tcp_t
---@field phase games.ConnPhase
---@field idle_ms integer
---@field watchdog uv.uv_timer_t
---@field heartbeat? uv.uv_timer_t

---@class games.Conn
---@field private opts games.ConnOpts
---@field private attempt? games.ConnAttempt
---@field private pending table<integer, games.ReplyCallback>
---@field private next_id integer
---@field private signals table<games.ConnEvent, games.Signal>
---@field private auto_reconnect boolean
---@field private backoff_ms integer
---@field private reconnect_timer? uv.uv_timer_t
---@field private last_disconnect? games.ErrorBody
---@field private credential_source table<games.CredentialField, games.CredentialSource>
---@field private signup_passphrase? string sent by the next login, which signs up in its place
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

---@param opts games.ConnOpts
---@return string|nil password
---@return string|nil problem why the conn cannot log in
local function resolve_login(opts)
  if opts.username == nil then
    return nil, "username is missing"
  end
  if opts.password == nil then
    return nil, "password is missing"
  end
  return resolve_password(opts.password)
end

---@param opts games.ConnOpts
---@return games.Conn
function Conn.new(opts)
  return setmetatable({
    opts = {
      host = opts.host,
      port = opts.port,
      username = opts.username,
      password = opts.password,
    },
    pending = {},
    next_id = 1,
    signals = {},
    auto_reconnect = false,
    backoff_ms = RECONNECT_MIN_MS,
    credential_source = {
      username = opts.username ~= nil and "config" or nil,
      password = opts.password ~= nil and "config" or nil,
    },
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

---A request's frame, without its newline. The body and its options are wire objects, so they
---encode as `{}` even when empty; every other empty table stays an array.
---@param id integer
---@param message_type games.MessageType
---@param body table|nil
---@return boolean encoded
---@return string frame the frame, or why it could not be encoded
function Conn.encode_request(id, message_type, body)
  local wire_body = json.object(body or {})
  if wire_body.options then
    wire_body = vim.tbl_extend("force", wire_body, { options = json.object(wire_body.options) })
  end
  ---@type games.Envelope
  local envelope = { id = id, type = message_type, body = wire_body }
  return pcall(vim.json.encode, envelope)
end

---Subscribes fn to a push type or a lifecycle event; fn runs via vim.schedule.
---@param event games.ConnEvent
---@param fn fun(payload: any)
---@overload fun(self: games.Conn, event: "connected", fn: fun(login: games.LoginResult))
---@overload fun(self: games.Conn, event: "disconnected", fn: fun(disconnect: games.Disconnect))
---@overload fun(self: games.Conn, event: "challenge.changed", fn: fun(challenge: games.Challenge))
---@overload fun(self: games.Conn, event: "game.update", fn: fun(update: games.GameUpdate))
function Conn:on(event, fn)
  self.signals[event] = self.signals[event] or Signal.new()
  self.signals[event].connect(fn)
end

---Connects and logs in, reconnecting with backoff until close() or a fatal login error.
function Conn:connect()
  self.last_disconnect = nil
  self.auto_reconnect = true
  self.backoff_ms = RECONNECT_MIN_MS
  if self.reconnect_timer then
    self.reconnect_timer:stop()
  end
  if not self.attempt then
    self:open()
  end
end

---Fills in missing credentials for this session only; a configured one is never replaced.
---A rejected login forgets the filled-in ones, so they can be asked for again.
---@param entered games.Credentials
function Conn:fill_credentials(entered)
  for _, field in ipairs(CREDENTIAL_FIELDS) do
    if self.opts[field] == nil and entered[field] ~= nil then
      self.opts[field] = entered[field]
      self.credential_source[field] = "prompt"
    end
  end
end

---Makes the next login create the account with the server's sign-up passphrase; every later
---one, reconnects included, logs in as usual.
---@param passphrase string
function Conn:sign_up(passphrase)
  self.signup_passphrase = passphrase
end

---@return games.CredentialField[] in the order to ask for them
function Conn:missing_credentials()
  return vim.tbl_filter(function(field)
    return self.opts[field] == nil
  end, CREDENTIAL_FIELDS)
end

---@return string host:port
function Conn:address()
  return ("%s:%d"):format(self.opts.host, self.opts.port)
end

---True from connect() until logged in, including the waits between reconnect tries.
---@return boolean
function Conn:is_connecting()
  return self.auto_reconnect and not (self.attempt and self.attempt.phase == Phase.AUTHENTICATED)
end

---Why the latest attempt or connection ended, or nil if none has.
---@return games.ErrorBody|nil
function Conn:last_error()
  return self.last_disconnect
end

---Sends a request once logged in; otherwise cb gets a "disconnected" error.
---@param message_type games.MessageType
---@param body table|nil
---@param cb games.ReplyCallback
function Conn:request(message_type, body, cb)
  local attempt = self.attempt
  if not (attempt and attempt.phase == Phase.AUTHENTICATED) then
    deliver(cb, disconnected("not connected"), nil)
    return
  end
  self:send_request(attempt, message_type, body, cb)
end

---Stops reconnecting and drops the connection.
function Conn:close()
  self.auto_reconnect = false
  timer.close(self.reconnect_timer)
  self.reconnect_timer = nil
  if self.attempt then
    self:teardown(self.attempt, disconnected("closed"))
  end
end

---@private
---@param event games.ConnEvent
---@param payload any
---@overload fun(self: games.Conn, event: "connected", payload: games.LoginResult)
---@overload fun(self: games.Conn, event: "disconnected", payload: games.Disconnect)
function Conn:emit(event, payload)
  local signal = self.signals[event]
  for _, fn in ipairs(signal and signal.handlers() or {}) do
    vim.schedule(function()
      fn(payload)
    end)
  end
end

---@private
function Conn:open()
  local password, problem = resolve_login(self.opts)
  if not password then
    self.auto_reconnect = false
    ---@cast problem string
    ---@type games.Disconnect
    local disconnect = { code = ErrorCode.AUTH_FAILED, message = problem, retrying = false }
    self.last_disconnect = disconnect
    self:emit("disconnected", disconnect)
    return
  end
  ---@type games.ConnAttempt
  local attempt = {
    buffer = "",
    phase = Phase.CONNECTING,
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
    self:try_address(attempt, addresses, 1, password)
  end)
end

---@private
---@param attempt games.ConnAttempt
---@param addresses uv.address[]
---@param index integer
---@param password string
function Conn:try_address(attempt, addresses, index, password)
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
    self:try_address(attempt, addresses, index + 1, password)
  end
  local _, connect_err = socket:connect(address.addr, address.port, function(err)
    if self.attempt ~= attempt then
      return
    end
    if err then
      try_next_address()
      return
    end
    attempt.phase = Phase.OPEN
    socket:nodelay(true)
    socket:read_start(function(read_err, chunk)
      self:receive(attempt, read_err, chunk)
    end)
    self:login(attempt, password)
  end)
  if connect_err then
    try_next_address()
  end
end

---@private
---@param attempt games.ConnAttempt
---@param password string
function Conn:login(attempt, password)
  ---@type games.LoginRequest|games.RegisterRequest
  local body = { v = types.PROTOCOL_VERSION, username = self.opts.username, password = password }
  local message_type = MessageType.AUTH_LOGIN
  local passphrase = self.signup_passphrase
  if passphrase then
    self.signup_passphrase = nil
    body = vim.tbl_extend("error", body, { passphrase = passphrase })
    message_type = MessageType.AUTH_REGISTER
  end
  self:send_request(attempt, message_type, body, function(err, result)
    if self.attempt ~= attempt then
      return
    end
    if err then
      if FATAL_LOGIN_ERRORS[err.code] then
        self.auto_reconnect = false
      end
      if CREDENTIALS_REJECTED[err.code] then
        self:forget_prompted_credentials()
      end
      self:teardown(attempt, err)
      return
    end
    ---@cast result games.LoginResult
    attempt.phase = Phase.AUTHENTICATED
    attempt.idle_ms = IDLE_HEARTBEATS * result.heartbeat_ms
    self.backoff_ms = RECONNECT_MIN_MS
    self:arm_watchdog(attempt)
    self:start_heartbeat(attempt, result.heartbeat_ms)
    self:emit("connected", result)
  end)
end

---@private
function Conn:forget_prompted_credentials()
  for _, field in ipairs(CREDENTIAL_FIELDS) do
    if self.credential_source[field] == "prompt" then
      self.opts[field] = nil
      self.credential_source[field] = nil
    end
  end
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
  local encoded, frame = Conn.encode_request(id, message_type, body)
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
  local ok, envelope = pcall(json.decode, line)
  if not ok or type(envelope) ~= "table" then
    self:teardown(attempt, disconnected("malformed frame from server"))
    return
  end
  ---@cast envelope games.Envelope
  if envelope.id == nil then
    self:emit(envelope.type --[[@as games.ConnEvent]], envelope.body)
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
  self.last_disconnect = reason
  timer.close(attempt.watchdog)
  timer.close(attempt.heartbeat)
  if attempt.socket and not attempt.socket:is_closing() then
    attempt.socket:close()
  end

  local pending = self.pending
  self.pending = {}
  for _, cb in pairs(pending) do
    deliver(cb, disconnected("connection lost"), nil)
  end
  if attempt.phase ~= Phase.CONNECTING then
    self:emit("disconnected", vim.tbl_extend("force", reason, { retrying = self.auto_reconnect }))
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
