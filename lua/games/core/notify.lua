local types = require("games.types")

local ChallengeState = types.ChallengeState
local ErrorCode = types.ErrorCode

local NOTIFY_OPTS = { title = "games" }
---@type table<games.ErrorCode, string> disconnect reasons that stop auto-reconnect
local FATAL_DISCONNECTS = {
  [ErrorCode.AUTH_FAILED] = "games: login failed (%s); not reconnecting, check username and password",
  [ErrorCode.VERSION_MISMATCH] = "games: server speaks another protocol version (%s); update the plugin",
}

---@class (private) games.NotifyState
---@field challenges_to_open table<games.ChallengeID, true> accepted from this nvim
---@field games_to_open table<games.GameID, true> started but not yet in the cache

local M = {}

---@type table<games.Client, games.NotifyState>
local states = setmetatable({}, { __mode = "k" })

---@param message string
---@param level? integer
local function notify(message, level)
  vim.notify(message, level or vim.log.levels.INFO, NOTIFY_OPTS)
end

---@param client games.Client
---@param game_id games.GameID
local function open_board(client, game_id)
  require("games.ui.shell").open(client, game_id, require("games").open_options())
end

---@param client games.Client
---@param state games.NotifyState
---@param game_id games.GameID
local function open_when_cached(client, state, game_id)
  if client:game(game_id) then
    open_board(client, game_id)
  else
    state.games_to_open[game_id] = true
  end
end

---@param client games.Client
---@param game_type games.GameType
---@param fn fun(name: string)
local function with_game_name(client, game_type, fn)
  client:catalog_list(nil, function(_, result)
    local info = result
      and vim.iter(result.games):find(function(game)
        return game.id == game_type
      end)
    fn(info and info.name or game_type)
  end)
end

---@alias games.ChallengeMessage fun(summary: string): string

---@param challenge games.Challenge resolved
---@param me games.UserID
---@return games.ChallengeMessage|nil
local function resolution_message(challenge, me)
  local owner = challenge.from == me and "your" or (challenge.from .. "'s")
  if challenge.state == ChallengeState.DECLINED and challenge.by ~= me then
    return function(summary)
      return ("%s declined %s %s challenge"):format(challenge.by, owner, summary)
    end
  end
  if challenge.state == ChallengeState.EXPIRED then
    return function(summary)
      return ("%s %s challenge expired"):format(owner, summary)
    end
  end
end

---@param client games.Client
---@param challenge games.Challenge
---@param message games.ChallengeMessage
local function notify_challenge(client, challenge, message)
  with_game_name(client, challenge.game, function(name)
    notify(message(require("games.ui.picker").game_summary(name, challenge.options)))
  end)
end

---@param client games.Client
---@param state games.NotifyState
---@param challenge games.Challenge
---@param is_new boolean
local function on_challenge(client, state, challenge, is_new)
  local me = client:me()
  if not me then
    return
  end
  local id = challenge.challenge_id
  if challenge.state == ChallengeState.PENDING then
    if is_new and challenge.from ~= me and vim.tbl_contains(challenge.to, me) then
      notify_challenge(client, challenge, function(summary)
        return ("%s challenges you: %s — :Games challenges"):format(challenge.from, summary)
      end)
    end
    return
  end
  local wanted = challenge.from == me or state.challenges_to_open[id]
  state.challenges_to_open[id] = nil
  if challenge.state == ChallengeState.ACCEPTED then
    if wanted and challenge.game_id then
      open_when_cached(client, state, challenge.game_id)
    end
    return
  end
  local message = resolution_message(challenge, me)
  if message then
    notify_challenge(client, challenge, message)
  end
end

---The notification for an update to a game no window shows, or nil when nothing happened.
---@param update games.GameUpdate
---@param me games.UserID
---@return string|nil
local function hidden_update_message(update, me)
  local shell = require("games.ui.shell")
  local description = shell.describe(update)
  if description == "" and not update.result then
    return nil
  end
  local status = update.result and shell.result_text(update.result)
    or (vim.tbl_contains(update.to_act, me) and "your move" or "waiting")
  local body = description == "" and status or (description .. " — " .. status)
  return ("%s: %s"):format(shell.title(update, me), body)
end

---A new game_id is either a game this nvim opens anyway or the initial sync at login, which is not news.
---@param client games.Client
---@param state games.NotifyState
---@param update games.GameUpdate
---@param is_new boolean
local function on_update(client, state, update, is_new)
  if state.games_to_open[update.game_id] then
    state.games_to_open[update.game_id] = nil
    open_board(client, update.game_id)
    return
  end
  local me = client:me()
  if is_new or not me or require("games.ui.shell").is_visible(update.game_id) then
    return
  end
  local message = hidden_update_message(update, me)
  if message then
    notify(message)
  end
end

---@param err games.ErrorBody
local function on_disconnected(err)
  local template = FATAL_DISCONNECTS[err.code]
  if template then
    notify(template:format(err.message), vim.log.levels.ERROR)
  end
end

---Turns client pushes into notifications and opens boards of games this nvim started.
---@param client games.Client
function M.attach(client)
  if states[client] then
    return
  end
  ---@type games.NotifyState
  local state = { challenges_to_open = {}, games_to_open = {} }
  states[client] = state
  client:on_challenge(function(challenge, is_new)
    on_challenge(client, state, challenge, is_new)
  end)
  client:on_update(function(update, is_new)
    on_update(client, state, update, is_new)
  end)
  client.conn:on("disconnected", on_disconnected)
end

---Opens the board when the challenge, accepted from this nvim, starts its game.
---@param client games.Client
---@param challenge_id games.ChallengeID
function M.open_when_started(client, challenge_id)
  local state = states[client]
  if state then
    state.challenges_to_open[challenge_id] = true
  end
end

return M
