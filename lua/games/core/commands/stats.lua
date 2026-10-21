local float = require("games.ui.float")
local kv = require("games.ui.widgets.kv")
local notify = require("games.core.notify")
local reply = require("games.core.commands.reply")
local stats = require("games.core.stats")

local M = {}

local on_ok = reply.on_ok

---@param title string
---@param empty_message string
---@param rows games.KvPair[]
local function show_rows(title, empty_message, rows)
  if #rows == 0 then
    notify.say(empty_message)
    return
  end
  float.open(title, kv({ pairs = rows }))
end

---Shows user's record in a float.
---@param client games.Client
---@param user games.UserID
function M.show_stats(client, user)
  client:stats_get(
    { user = user },
    on_ok(
      ---@param result games.StatsResult
      function(result)
        show_rows(
          " stats: " .. user .. " ",
          ("%s has no finished games"):format(user),
          stats.stats_pairs(result)
        )
      end
    )
  )
end

---Shows the server's ranking in a float, of game's records or, when nil, of every game.
---@param client games.Client
---@param game games.GameType|nil
function M.show_leaderboard(client, game)
  client:stats_get(
    nil,
    on_ok(
      ---@param result games.StatsResult
      function(result)
        show_rows(
          (" leaderboard: %s "):format(game or "all games"),
          ("no finished %sgames"):format(game and (game .. " ") or ""),
          stats.leaderboard_pairs(result, game)
        )
      end
    )
  )
end

---@type games.SubcommandRun
function M.run_stats(client, me, args)
  M.show_stats(client, args[1] or me)
end

---@type games.SubcommandRun
function M.run_leaderboard(client, _, args)
  M.show_leaderboard(client, args[1])
end

return M
