local notify = require("games.core.notify")
local timer = require("games.core.timer")

---One full connect attempt (a 10s watchdog) plus a retry; a command firing later than this would
---surprise a user who has moved on.
local PENDING_TIMEOUT_MS = 15000

---@type table<games.CredentialField, string>
local PROMPTS = { username = "games username: ", password = "games password: " }

---@class (private) games.PendingRun
---@field label string how the user typed the command
---@field run fun()
---@field timer? uv.uv_timer_t armed once the conn is connecting for it

---@class (private) games.LoginState
---@field pending? games.PendingRun the newest command waiting for a connection
---@field prompting boolean a credential prompt is open

local M = {}

---@type table<games.Client, games.LoginState>
local states = setmetatable({}, { __mode = "k" })

---@param state games.LoginState
---@return games.PendingRun|nil
local function take_pending(state)
  local pending = state.pending
  state.pending = nil
  if pending then
    timer.close(pending.timer)
  end
  return pending
end

---@param state games.LoginState
---@param label string
---@param run fun()
local function hold(state, label, run)
  local replaced = take_pending(state)
  if replaced then
    notify.say(
      ("still connecting — replaced pending `%s` with `%s`"):format(replaced.label, label)
    )
  end
  state.pending = { label = label, run = run }
end

---@param client games.Client
---@return games.LoginState
local function state_of(client)
  local state = states[client]
  if state then
    return state
  end
  state = { prompting = false }
  states[client] = state
  client.conn:on("connected", function()
    local pending = take_pending(state)
    if pending then
      pending.run()
    end
  end)
  client.conn:on("disconnected", function(disconnect)
    if not disconnect.retrying then
      take_pending(state)
    end
  end)
  return state
end

---@param client games.Client
---@param state games.LoginState
---@param pending games.PendingRun
local function arm_timeout(client, state, pending)
  pending.timer = assert(vim.uv.new_timer())
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
      notify.error(("could not reach server %s%s"):format(client.conn:address(), reason))
    end)
  )
end

---@param field games.CredentialField
---@param on_value fun(value: string|nil)
local function ask(field, on_value)
  if field == "password" then
    local ok, password = pcall(vim.fn.inputsecret, PROMPTS.password)
    on_value(ok and password or nil)
    return
  end
  vim.ui.input({ prompt = PROMPTS[field] }, on_value)
end

---Asks for each field in turn; on_done gets them all, or nil once one is cancelled or empty.
---@param fields games.CredentialField[]
---@param on_done fun(entered: games.Credentials|nil)
local function prompt_credentials(fields, on_done)
  ---@param index integer
  ---@param entered games.Credentials
  local function step(index, entered)
    local field = fields[index]
    if not field then
      on_done(entered)
      return
    end
    ask(field, function(value)
      if not value or value == "" then
        on_done(nil)
        return
      end
      step(index + 1, vim.tbl_extend("force", entered, { [field] = value }))
    end)
  end
  step(1, {})
end

---Moves the pending command on: runs it, asks for missing credentials, or connects for it.
---@param client games.Client
---@param state games.LoginState
local function advance(client, state)
  local pending = state.pending
  if not pending or state.prompting then
    return
  end
  if client:me() then
    take_pending(state)
    pending.run()
    return
  end
  local missing = client.conn:missing_credentials()
  if #missing > 0 then
    state.prompting = true
    prompt_credentials(missing, function(entered)
      state.prompting = false
      if not entered then
        take_pending(state)
        return
      end
      client.conn:fill_credentials(entered)
      advance(client, state)
    end)
    return
  end
  if not pending.timer then
    arm_timeout(client, state, pending)
  end
  client.conn:connect()
end

---Runs run now if logged in; otherwise asks for missing credentials, connects, and runs it once
---connected. Only the newest waiting run is kept; a fatal disconnect or a timeout drops it.
---@param client games.Client
---@param label string how the user typed the command, for the notice when a newer one replaces it
---@param run fun()
function M.ensure_connected(client, label, run)
  if client:me() then
    run()
    return
  end
  local state = state_of(client)
  hold(state, label, run)
  advance(client, state)
end

return M
