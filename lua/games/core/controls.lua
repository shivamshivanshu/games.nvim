local describe = require("games.core.describe")
local notify = require("games.core.notify")
local prompt = require("games.ui.prompt")

local M = {}

---Reports a failed request about update's game, naming the game as me sees it.
---@param update games.GameUpdate
---@param me games.UserID
---@return games.ReplyCallback
function M.report_failure(update, me)
  return function(err)
    if err then
      notify.say(("%s: %s"):format(describe.title(update, me), err.message), vim.log.levels.ERROR)
    end
  end
end

---Reports a failed request in the server's words alone.
---@type games.ReplyCallback
function M.report_message(err)
  if err then
    notify.say(err.message, vim.log.levels.ERROR)
  end
end

---Sends control for update's game at the seq the client holds by then, once wording's
---confirmation, if it has one, is given.
---@param client games.Client
---@param update games.GameUpdate
---@param control games.Control
---@param wording games.ControlWording
---@param on_reply games.ReplyCallback
function M.send(client, update, control, wording, on_reply)
  if wording.confirm and not prompt.confirm(wording.confirm) then
    return
  end
  local latest = client:game(update.game_id) or update
  client:game_control({ game_id = latest.game_id, seq = latest.seq, control = control }, on_reply)
end

return M
