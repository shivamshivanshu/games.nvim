local challenge = require("games.core.commands.challenge")
local notify = require("games.core.notify")
local players = require("games.core.players")
local reply = require("games.core.commands.reply")
local shell = require("games.ui.shell")

local M = {}

local on_ok = reply.on_ok

---Starts the solo game info, once its options are answered, and opens its board.
---@param client games.Client
---@param info games.GameInfo
local function start_solo(client, info)
  challenge.pick_options(info, function(options)
    client:game_start(
      { game = info.id, options = options },
      on_ok(
        ---@param update games.GameUpdate
        function(update)
          shell.open(client, update.game_id)
        end
      )
    )
  end)
end

---Starts a solo game on its own and challenges for any other, asking for its options first.
---@param client games.Client
---@param me games.UserID
---@param info games.GameInfo
function M.start(client, me, info)
  if not players.is_solo_game(info) then
    challenge.challenge(client, me, info.id, {})
    return
  end
  start_solo(client, info)
end

---Starts the solo game named by args[1], or one picked from the catalogue's solo games.
---@type games.SubcommandRun
function M.run(client, _, args)
  local game_type = args[1]
  client:catalog_list(
    nil,
    on_ok(function()
      if not game_type then
        local solo_games = vim.tbl_filter(players.can_start_solo, client:catalog_games())
        if #solo_games == 0 then
          notify.error("no solo games available")
          return
        end
        challenge.pick_info(solo_games, "Solo game:", function(info)
          start_solo(client, info)
        end)
        return
      end
      local info = client:game_info(game_type)
      if not info then
        notify.error(("no game %q"):format(game_type))
        return
      end
      if not players.can_start_solo(info) then
        notify.error(
          ("%s is not a solo game; challenge someone with :Games challenge %s"):format(
            game_type,
            game_type
          )
        )
        return
      end
      start_solo(client, info)
    end)
  )
end

---The solo games of the catalogue the plugin's client has fetched, for completion.
---@return games.GameType[]
function M.game_types()
  local client = require("games").client
  local solo_games = client and vim.tbl_filter(players.can_start_solo, client:catalog_games()) or {}
  return vim.tbl_map(function(info)
    return info.id
  end, solo_games)
end

return M
