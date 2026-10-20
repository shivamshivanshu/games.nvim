local notify = require("games.core.notify")
local types = require("games.types")

local ErrorCode = types.ErrorCode

---One full connect attempt (a 10s watchdog) plus a retry; a command firing later than this would
---surprise a user who has moved on.
local PENDING_TIMEOUT_MS = 15000

---@class (private) games.PendingRun
---@field run fun()
---@field timer uv.uv_timer_t

---@class (private) games.LoginState
---@field pending? games.PendingRun the newest command waiting for a connection
---@field typed_credentials boolean the conn holds credentials typed at a prompt and not yet accepted

local M = {}

---@type table<games.Client, games.LoginState>
local states = setmetatable({}, { __mode = "k" })

---@param state games.LoginState
---@return games.PendingRun|nil
local function take_pending(state)
  local pending = state.pending
  state.pending = nil
  if pending and not pending.timer:is_closing() then
    pending.timer:stop()
    pending.timer:close()
  end
  return pending
end

---A rejected typed password is forgotten, so the next :Games asks again.
---@param client games.Client
---@param state games.LoginState
---@param disconnect games.Disconnect
local function on_disconnected(client, state, disconnect)
  if disconnect.retrying then
    return
  end
  take_pending(state)
  if state.typed_credentials and disconnect.code == ErrorCode.AUTH_FAILED then
    client.conn:set_credentials(client.conn:username(), nil)
  end
  state.typed_credentials = false
end

---@param client games.Client
---@return games.LoginState
local function state_of(client)
  local state = states[client]
  if state then
    return state
  end
  state = { typed_credentials = false }
  states[client] = state
  client.conn:on("connected", function()
    state.typed_credentials = false
    local pending = take_pending(state)
    if pending then
      pending.run()
    end
  end)
  client.conn:on("disconnected", function(disconnect)
    on_disconnected(client, state, disconnect)
  end)
  return state
end

---@param client games.Client
---@param run fun()
local function await_connection(client, run)
  local state = state_of(client)
  take_pending(state)
  ---@type games.PendingRun
  local pending = { run = run, timer = assert(vim.uv.new_timer()) }
  state.pending = pending
  pending.timer:start(
    PENDING_TIMEOUT_MS,
    0,
    vim.schedule_wrap(function()
      if state.pending ~= pending then
        return
      end
      take_pending(state)
      local last_error = client.conn:last_error()
      local reason = last_error and (": " .. last_error.message) or ""
      notify.say(
        ("games: could not reach server %s%s"):format(client.conn:address(), reason),
        vim.log.levels.ERROR
      )
    end)
  )
  client.conn:connect()
end

---Asks for the username, then the password; on_entered runs only when both are non-empty.
---@param conn games.Conn
---@param on_entered fun()
local function prompt_credentials(conn, on_entered)
  vim.ui.input({ prompt = "games username: ", default = conn:username() }, function(username)
    if not username or username == "" then
      return
    end
    local ok, password = pcall(vim.fn.inputsecret, "games password: ")
    if not ok or password == "" then
      return
    end
    conn:set_credentials(username, password)
    on_entered()
  end)
end

---Runs run now if logged in; otherwise asks for missing credentials, connects, and runs it once
---connected. Only the newest waiting run is kept; a fatal disconnect or a timeout drops it.
---@param client games.Client
---@param run fun()
function M.ensure_connected(client, run)
  if client:me() then
    run()
    return
  end
  if client.conn:has_credentials() then
    await_connection(client, run)
    return
  end
  prompt_credentials(client.conn, function()
    state_of(client).typed_credentials = true
    await_connection(client, run)
  end)
end

return M
