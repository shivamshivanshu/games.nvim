local describe = require("games.core.describe")
local players = require("games.core.players")
local summary = require("games.core.summary")
local types = require("games.types")

local ChallengeState = types.ChallengeState
local ErrorCode = types.ErrorCode

local NOTIFY_OPTS = { title = "games" }
local PREFIX = "games: "
---@type table<games.ErrorCode, string> what to say when the conn gives up after this error
local GIVE_UP_MESSAGES = {
  [ErrorCode.AUTH_FAILED] = "login failed (%s); not reconnecting, check username and password",
  [ErrorCode.VERSION_MISMATCH] = "server speaks another protocol version (%s); update the plugin",
  [ErrorCode.SIGNUP_DISABLED] = "sign-up failed (%s); ask the server's host for an account",
  [ErrorCode.PASSPHRASE_REJECTED] = "sign-up failed (%s); check the passphrase with the server's host",
  [ErrorCode.USER_EXISTS] = "sign-up failed (%s); pick another username",
}

---@class (private) games.NotifyState
---@field challenges_to_open table<games.ChallengeID, true> accepted from this nvim
---@field games_to_open table<games.GameID, true> started but not yet in the cache

local M = {}

---@type table<games.Client, games.NotifyState>
local states = setmetatable({}, { __mode = "k" })

---vim.notify under the plugin's title and after its prefix; level defaults to INFO.
---@param message string
---@param level? integer
function M.say(message, level)
  vim.notify(PREFIX .. message, level or vim.log.levels.INFO, NOTIFY_OPTS)
end

---@param message string
function M.error(message)
  M.say(message, vim.log.levels.ERROR)
end

---@param message string
function M.warn(message)
  M.say(message, vim.log.levels.WARN)
end

---The logged-in user; while disconnected it warns and returns nil.
---@param client games.Client
---@return games.UserID|nil
function M.require_me(client)
  local me = client:me()
  if not me then
    M.warn("not connected")
  end
  return me
end

---@param client games.Client
---@param game_id games.GameID
local function open_board(client, game_id)
  require("games.ui.shell").open(client, game_id)
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

---@alias games.ChallengeMessage fun(summary: string): string

---How a resolved challenge is reported to me, or nil when I resolved it myself.
---@param challenge games.Challenge resolved
---@param me games.UserID
---@return games.ChallengeMessage|nil
function M.resolution_message(challenge, me)
  local mine = challenge.from == me
  local owner = mine and "your" or (challenge.from .. "'s")
  local by_other = challenge.by ~= nil and challenge.by ~= me
  if challenge.state == ChallengeState.DECLINED and by_other then
    return function(summary)
      return ("%s declined %s %s challenge"):format(challenge.by, owner, summary)
    end
  end
  if challenge.state == ChallengeState.CANCELLED and by_other then
    return function(summary)
      return ("%s cancelled the %s challenge"):format(challenge.by, summary)
    end
  end
  if challenge.state == ChallengeState.EXPIRED then
    local invitees = mine and (" to " .. table.concat(challenge.to, ", ")) or ""
    return function(summary)
      return ("%s %s challenge%s expired"):format(owner, summary, invitees)
    end
  end
end

---@param client games.Client
---@param challenge games.Challenge
---@param message games.ChallengeMessage
local function notify_challenge(client, challenge, message)
  client:game_infos(function(infos)
    M.say(message(summary.game_summary(challenge.game, infos[challenge.game], challenge.options)))
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
  local message = M.resolution_message(challenge, me)
  if message then
    notify_challenge(client, challenge, message)
  end
end

---The notification for an update to a game no window shows, or nil when nothing happened.
---@param update games.GameUpdate
---@param me games.UserID
---@return string|nil
local function hidden_update_message(update, me)
  local description = describe.events(update)
  if description == "" and not update.result then
    return nil
  end
  local status = update.result and describe.result_text(update.result)
    or describe.turn_text(update, me)
  local body = description == "" and status or (description .. " — " .. status)
  return ("%s: %s"):format(describe.title(update, me), body)
end

---@param update games.GameUpdate
---@param me games.UserID
---@return boolean
local function only_my_actions(update, me)
  return #update.events > 0
    and vim.iter(update.events):all(function(event)
      return event.by ~= nil and players.user_of_seat(update, event.by) == me
    end)
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
  if not update.result and only_my_actions(update, me) then
    return
  end
  local message = hidden_update_message(update, me)
  if message then
    M.say(message)
  end
end

---@param disconnect games.Disconnect
local function on_disconnected(disconnect)
  local template = not disconnect.retrying and GIVE_UP_MESSAGES[disconnect.code]
  if template then
    M.error(template:format(disconnect.message))
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
