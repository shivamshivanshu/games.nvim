local players = require("games.core.players")
local registry = require("games.core.registry")
local text = require("games.ui.text")
local types = require("games.types")

local SHORT_ID_LENGTH = 4

local M = {}

---@param update games.GameUpdate
---@param me games.UserID
---@param separator string
---@return string
function M.opponent_names(update, me, separator)
  local opponents = vim
    .iter(update.players)
    :map(function(player)
      return player.user
    end)
    :filter(function(user)
      return user ~= me
    end)
    :totable()
  return table.concat(opponents, separator)
end

---The id prefix that buffer names and pickers show.
---@param game_id games.GameID
---@return string
function M.short_id(game_id)
  return game_id:sub(1, SHORT_ID_LENGTH)
end

---"<game> vs <opponents>", the name notifications and errors use for a game.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.title(update, me)
  return ("%s vs %s"):format(update.game, M.opponent_names(update, me, ", "))
end

---@param update games.GameUpdate with a clock
---@param user games.UserID
---@param elapsed_ms integer charged to user's clock if it is the running one
---@return string
function M.clock_text(update, user, elapsed_ms)
  local remaining_ms = update.clock.remaining_ms[user] or 0
  if update.clock.running == user then
    remaining_ms = remaining_ms - elapsed_ms
  end
  local seconds = math.max(0, math.ceil(remaining_ms / 1000))
  return ("%d:%02d"):format(math.floor(seconds / 60), seconds % 60)
end

---@param result games.WireResult
---@return string
function M.result_text(result)
  local reason = text.humanize(result.reason)
  if result.kind == types.ResultKind.ABORTED then
    return "Result: aborted"
  end
  if result.kind == types.ResultKind.DRAW then
    return ("Result: draw (%s)"):format(reason)
  end
  if #result.winners == 0 then
    return ("Result: lost (%s)"):format(reason)
  end
  return ("Result: %s won (%s)"):format(table.concat(result.winners, ", "), reason)
end

---"your move", "waiting" or "finished", as me sees the game.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.turn_text(update, me)
  if update.result then
    return "finished"
  end
  return players.is_to_act(update, me) and "your move" or "waiting"
end

---@param event games.Event
---@param update games.GameUpdate
---@return string
local function fallback_description(event, update)
  local actor = event.by and (players.user_of_seat(update, event.by) or event.by)
  return actor and ("%s: %s"):format(actor, event.kind) or event.kind
end

---A game module that fails to load falls back to generic descriptions, so listings still work.
---@param update games.GameUpdate
---@return string
function M.events(update)
  local loaded, module = pcall(registry.game_module, update.game)
  local describe_event = loaded and module and module.describe_event
  local descriptions = vim.tbl_map(function(event)
    return describe_event and describe_event(event, update) or fallback_description(event, update)
  end, update.events)
  return table.concat(descriptions, "; ")
end

return M
