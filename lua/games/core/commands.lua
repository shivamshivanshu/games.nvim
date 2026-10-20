local float = require("games.ui.float")
local kv = require("games.ui.widgets.kv")
local login = require("games.core.login")
local notify = require("games.core.notify")
local picker = require("games.ui.picker")
local shell = require("games.ui.shell")
local stats = require("games.core.stats")
local text = require("games.ui.text")
local types = require("games.types")

local GameStatus = types.GameStatus
local ERROR = vim.log.levels.ERROR
local CHALLENGE_PLAYERS = 2
local GAME_STATUSES = { GameStatus.ONGOING, GameStatus.FINISHED }

---@alias games.SubcommandRun fun(client: games.Client, me: games.UserID, args: string[])

---@class (private) games.Subcommand
---@field name string
---@field usage string
---@field run games.SubcommandRun
---@field arg_choices? string[] completions for the first argument

---@class (private) games.ChallengeAction
---@field label string
---@field send fun(client: games.Client, ref: games.ChallengeRef, cb: games.ReplyCallback)

local M = {}

local say = notify.say

---Reply callback that reports request errors and hands successful results on.
---@generic R
---@param on_result fun(result: R)
---@return games.Reply<R>
local function on_ok(on_result)
  return function(err, result)
    if err then
      say("games: " .. err.message, ERROR)
      return
    end
    on_result(result)
  end
end

local function ignore() end

---@param client games.Client
---@param me games.UserID
---@param on_user fun(user: games.UserID)
local function pick_opponent(client, me, on_user)
  client:user_list(
    nil,
    on_ok(
      ---@param result games.UserListResult
      function(result)
        local others = vim.tbl_filter(function(presence)
          return presence.user ~= me
        end, result.users)
        if #others == 0 then
          say("games: no other users")
          return
        end
        table.sort(others, function(a, b)
          if a.online ~= b.online then
            return a.online
          end
          return a.user < b.user
        end)
        picker.pick(others, {
          prompt = "Challenge whom?",
          format_item = function(presence)
            return presence.online and (presence.user .. " (online)") or presence.user
          end,
        }, function(presence)
          on_user(presence.user)
        end)
      end
    )
  )
end

---@param games games.GameInfo[]
---@param game_type games.GameType|nil picked interactively when nil
---@param on_game fun(info: games.GameInfo)
local function pick_game(games, game_type, on_game)
  local playable = vim.tbl_filter(function(info)
    return info.players.min <= CHALLENGE_PLAYERS and CHALLENGE_PLAYERS <= info.players.max
  end, games)
  if #playable == 0 then
    say("games: no games available")
    return
  end
  if not game_type then
    picker.pick(playable, {
      prompt = "Game:",
      format_item = function(info)
        return info.name
      end,
    }, on_game)
    return
  end
  local info = vim.iter(playable):find(function(candidate)
    return candidate.id == game_type
  end)
  if not info then
    say(("games: no two-player game %q"):format(game_type), ERROR)
    return
  end
  on_game(info)
end

---@param option games.GameOption
---@return string[] values with the default first, so accepting the first item takes it
local function default_first(option)
  return vim.list_extend(
    { option.default },
    vim.tbl_filter(function(value)
      return value ~= option.default
    end, option.values)
  )
end

---Asks for each option in name order; on_done gets every option once all are answered.
---@param info games.GameInfo
---@param on_done fun(options: games.Options)
local function pick_options(info, on_done)
  local names = vim.tbl_keys(info.options)
  table.sort(names)
  ---@param index integer
  ---@param chosen games.Options
  local function ask(index, chosen)
    local name = names[index]
    if not name then
      on_done(chosen)
      return
    end
    local option = info.options[name]
    picker.pick(default_first(option), {
      prompt = text.humanize(name) .. ":",
      format_item = function(value)
        return value == option.default and (value .. " (default)") or value
      end,
    }, function(value)
      ask(index + 1, vim.tbl_extend("force", chosen, { [name] = value }))
    end)
  end
  ask(1, {})
end

---@type games.SubcommandRun
local function challenge(client, me, args)
  ---@param opponent games.UserID
  local function challenge_opponent(opponent)
    client:catalog_list(
      nil,
      on_ok(
        ---@param catalog games.CatalogResult
        function(catalog)
          pick_game(catalog.games, args[2], function(info)
            pick_options(info, function(options)
              client:challenge_create(
                { to = { opponent }, game = info.id, options = options },
                on_ok(function()
                  say(
                    ("challenged %s: %s"):format(opponent, picker.game_summary(info.name, options))
                  )
                end)
              )
            end)
          end)
        end
      )
    )
  end
  if args[1] then
    challenge_opponent(args[1])
  else
    pick_opponent(client, me, challenge_opponent)
  end
end

---@type games.ChallengeAction[]
local INCOMING_ACTIONS = {
  {
    label = "accept",
    send = function(client, ref, cb)
      notify.open_when_started(client, ref.challenge_id)
      client:challenge_accept(ref, cb)
    end,
  },
  {
    label = "decline",
    send = function(client, ref, cb)
      client:challenge_decline(ref, cb)
    end,
  },
}

---@type games.ChallengeAction[]
local OUTGOING_ACTIONS = {
  {
    label = "cancel",
    send = function(client, ref, cb)
      client:challenge_cancel(ref, cb)
    end,
  },
}

---@param client games.Client
---@param me games.UserID
---@param picked games.Challenge
local function resolve_challenge(client, me, picked)
  local actions = picked.from == me and OUTGOING_ACTIONS or INCOMING_ACTIONS
  picker.pick(actions, {
    prompt = "Challenge:",
    format_item = function(action)
      return action.label
    end,
  }, function(action)
    action.send(client, { challenge_id = picked.challenge_id }, on_ok(ignore))
  end)
end

---@param me games.UserID
---@return fun(a: games.Challenge, b: games.Challenge): boolean
local function incoming_first(me)
  return function(a, b)
    local a_incoming, b_incoming = a.from ~= me, b.from ~= me
    if a_incoming ~= b_incoming then
      return a_incoming
    end
    return a.expires_at < b.expires_at
  end
end

---@type games.SubcommandRun
local function challenges(client, me)
  client:challenge_list(
    nil,
    on_ok(function()
      local pending = client:pending_challenges()
      if #pending == 0 then
        say("games: no pending challenges")
        return
      end
      table.sort(pending, incoming_first(me))
      client:game_names(function(names)
        picker.pick(pending, {
          prompt = "Pending challenges:",
          format_item = function(item)
            return picker.format_challenge(item, me, names[item.game] or item.game)
          end,
        }, function(item)
          resolve_challenge(client, me, item)
        end)
      end)
    end)
  )
end

---@type games.SubcommandRun
local function list(client, me, args)
  local status = args[1] or GameStatus.ONGOING
  if not vim.tbl_contains(GAME_STATUSES, status) then
    say(("games: list takes %s, not %q"):format(table.concat(GAME_STATUSES, " or "), status), ERROR)
    return
  end
  client:game_list(
    { status = status },
    on_ok(
      ---@param result games.GameListResult
      function(result)
        if #result.games == 0 then
          say(("games: no %s games"):format(status))
          return
        end
        picker.pick(result.games, {
          prompt = ("%s games:"):format(status),
          format_item = function(update)
            return picker.format_game(update, me)
          end,
        }, function(update)
          shell.open(client, update.game_id)
        end)
      end
    )
  )
end

---@param title string
---@param empty_message string
---@param rows games.KvPair[]
local function show_rows(title, empty_message, rows)
  if #rows == 0 then
    say(empty_message)
    return
  end
  float.open(title, kv({ pairs = rows }))
end

---@type games.SubcommandRun
local function show_stats(client, me, args)
  local user = args[1] or me
  client:stats_get(
    nil,
    on_ok(
      ---@param result games.StatsResult
      function(result)
        show_rows(
          " stats: " .. user .. " ",
          ("games: %s has no finished games"):format(user),
          stats.stats_pairs(result, user)
        )
      end
    )
  )
end

---@type games.SubcommandRun
local function show_leaderboard(client, _, args)
  local game = args[1]
  client:stats_get(
    nil,
    on_ok(
      ---@param result games.StatsResult
      function(result)
        show_rows(
          (" leaderboard: %s "):format(game or "all games"),
          ("games: no finished %sgames"):format(game and (game .. " ") or ""),
          stats.leaderboard_pairs(result.records, game)
        )
      end
    )
  )
end

---@type games.SubcommandRun
local function next_game(client)
  shell.open_next(client)
end

---@type games.Subcommand[]
local SUBCOMMANDS = {
  { name = "challenge", usage = "challenge [user] [game]", run = challenge },
  { name = "challenges", usage = "challenges", run = challenges },
  { name = "list", usage = "list [ongoing|finished]", run = list, arg_choices = GAME_STATUSES },
  { name = "stats", usage = "stats [user]", run = show_stats },
  { name = "leaderboard", usage = "leaderboard [game]", run = show_leaderboard },
  { name = "next", usage = "next", run = next_game },
}

---@param name string
---@return games.Subcommand|nil
local function find_subcommand(name)
  return vim.iter(SUBCOMMANDS):find(function(subcommand)
    return subcommand.name == name
  end)
end

---@param client games.Client
---@param subcommand games.Subcommand
---@param args string[]
local function run_subcommand(client, subcommand, args)
  login.ensure_connected(client, function()
    local me = notify.require_me(client)
    if me then
      subcommand.run(client, me, args)
    end
  end)
end

---Runs `:Games <fargs>`; with no subcommand it offers a menu of them.
---@param fargs string[]
function M.run(fargs)
  local client = require("games").client
  if not client then
    say('games: call require("games").setup() first', ERROR)
    return
  end
  local name = fargs[1]
  if not name then
    picker.pick(SUBCOMMANDS, {
      prompt = "Games:",
      format_item = function(subcommand)
        return subcommand.usage
      end,
    }, function(subcommand)
      run_subcommand(client, subcommand, {})
    end)
    return
  end
  local subcommand = find_subcommand(name)
  if not subcommand then
    say(("games: unknown subcommand %q"):format(name), ERROR)
    return
  end
  run_subcommand(client, subcommand, vim.list_slice(fargs, 2))
end

---Completes subcommand names, then the first argument where its choices are fixed.
---@param arg_lead string
---@param cmd_line string
---@param cursor_pos integer
---@return string[]
function M.complete(arg_lead, cmd_line, cursor_pos)
  local words = vim.split(cmd_line:sub(1, cursor_pos), "%s+", { trimempty = true })
  local completing = #words + (arg_lead == "" and 1 or 0)
  local candidates = {}
  if completing == 2 then
    candidates = vim.tbl_map(function(subcommand)
      return subcommand.name
    end, SUBCOMMANDS)
  elseif completing == 3 then
    local subcommand = find_subcommand(words[2])
    candidates = subcommand and subcommand.arg_choices or {}
  end
  return vim.tbl_filter(function(candidate)
    return vim.startswith(candidate, arg_lead)
  end, candidates)
end

return M
