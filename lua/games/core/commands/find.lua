local finder = require("games.ui.finder")
local notify = require("games.core.notify")
local picker = require("games.ui.picker")
local puzzles = require("games.core.puzzles")
local reply = require("games.core.commands.reply")
local sources = require("games.core.sources")
local types = require("games.types")

local GameStatus = types.GameStatus
local ERROR = vim.log.levels.ERROR

local M = {}

---@type games.GameStatus[]
M.GAME_STATUSES = { GameStatus.ONGOING, GameStatus.FINISHED }

local say = notify.say
local on_ok = reply.on_ok

---@alias (private) games.ExtraFetch fun(client: games.Client, done: fun(extra: games.SourceExtra))

---What each source needs fetched beyond the client cache; a source not listed needs nothing.
---@type table<games.SourceName, games.ExtraFetch>
local EXTRA_FETCHES = {
  [sources.SourceName.GAMES] = function(client, done)
    client:finished_games(function(finished)
      done({ finished = finished })
    end)
  end,
  [sources.SourceName.TYPES] = function(client, done)
    client:catalog_list(
      nil,
      on_ok(function()
        done({})
      end)
    )
  end,
  [sources.SourceName.REQUESTS] = function(client, done)
    client:challenge_list(
      nil,
      on_ok(function()
        client:game_infos(function()
          done({})
        end)
      end)
    )
  end,
  [sources.SourceName.PUZZLES] = function(_, done)
    puzzles.summaries(function(summaries)
      done({ puzzle_sets = summaries })
    end)
  end,
  [sources.SourceName.PLAYERS] = function(client, done)
    client:user_list(
      nil,
      on_ok(
        ---@param listed games.UserListResult
        function(listed)
          client:stats_get(
            nil,
            on_ok(
              ---@param result games.StatsResult
              function(result)
                done({ users = listed.users, stats = result })
              end
            )
          )
        end
      )
    )
  end,
}

---@param source games.Source
local function open_finder(source)
  finder.open(source, assert(require("games").config).picker)
end

---Opens the finder over source name once what it lists is fetched.
---@param client games.Client
---@param me games.UserID
---@param name games.SourceName
function M.find(client, me, name)
  local fetch = EXTRA_FETCHES[name] or function(_, done)
    done({})
  end
  fetch(client, function(extra)
    open_finder(sources.build(name, sources.snapshot(client, me, extra)))
  end)
end

---Opens the finder over the games of status args[1], ongoing by default, as game.list returns them,
---once the catalog naming their games is in.
---@type games.SubcommandRun
function M.run_list(client, me, args)
  local status = args[1] or GameStatus.ONGOING
  if not vim.tbl_contains(M.GAME_STATUSES, status) then
    say(("list takes %s, not %q"):format(table.concat(M.GAME_STATUSES, " or "), status), ERROR)
    return
  end
  client:game_list(
    { status = status },
    on_ok(
      ---@param result games.GameListResult
      function(result)
        client:game_infos(function()
          local snapshot =
            vim.tbl_extend("force", sources.snapshot(client, me), { games = result.games })
          open_finder(sources.build(sources.SourceName.GAMES, snapshot))
        end)
      end
    )
  )
end

---@type games.SubcommandRun
function M.run(client, me, args)
  local name = args[1]
  if not name then
    picker.pick(sources.NAMES, { prompt = "Find:" }, function(picked)
      M.find(client, me, picked)
    end)
    return
  end
  if not sources.is_source_name(name) then
    say(("find takes %s, not %q"):format(table.concat(sources.NAMES, ", "), name), ERROR)
    return
  end
  M.find(client, me, name)
end

return M
