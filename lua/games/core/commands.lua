local Archive = require("games.core.archive")
local dashboard = require("games.ui.dashboard")
local finder = require("games.ui.finder")
local float = require("games.ui.float")
local kv = require("games.ui.widgets.kv")
local login = require("games.core.login")
local notify = require("games.core.notify")
local picker = require("games.ui.picker")
local players = require("games.core.players")
local predicates = require("games.core.predicates")
local prompt = require("games.ui.prompt")
local puzzles = require("games.core.puzzles")
local shell = require("games.ui.shell")
local sources = require("games.core.sources")
local stats = require("games.core.stats")
local summary = require("games.core.summary")
local tables = require("games.core.tables")
local text = require("games.ui.text")
local timer = require("games.core.timer")
local types = require("games.types")

local GameStatus = types.GameStatus
local OptionType = types.OptionType
local ERROR = vim.log.levels.ERROR
local GAME_STATUSES = { GameStatus.ONGOING, GameStatus.FINISHED }
---@type games.GameType
local PUZZLE_GAME = "chess_puzzle"

---@alias games.SubcommandRun fun(client: games.Client, me: games.UserID, args: string[])

---@class (private) games.Subcommand
---@field name string
---@field usage string
---@field run? games.SubcommandRun
---@field run_offline? fun(args: string[]) in place of run, for a subcommand that needs no server
---@field arg_choices? fun(arg_lead: string): string[] completions for the first argument

---@class (private) games.ChallengeAction
---@field label string
---@field send fun(client: games.Client, ref: games.ChallengeRef, cb: games.ReplyCallback)

local M = {}

local say = notify.say

---Reply callback that reports request errors and hands successful results on, if asked to.
---@generic R
---@param on_result? fun(result: R)
---@return games.Reply<R>
local function on_ok(on_result)
  return function(err, result)
    if err then
      say(err.message, ERROR)
      return
    end
    if on_result then
      on_result(result)
    end
  end
end

---@class (private) games.OpponentChoice
---@field label string
---@field user? games.UserID nil for the choice that ends picking

---@type games.OpponentChoice
local DONE_PICKING = { label = "done" }

---@param presence games.UserPresence
---@return games.OpponentChoice
local function opponent_choice(presence)
  return { label = summary.format_presence(presence), user = presence.user }
end

---@class (private) games.OpponentRange
---@field min integer fewest opponents, before which "done" is not offered
---@field max integer most opponents, at which picking stops

---Asks for one opponent at a time, offering "done" once range.min are picked, until done or range.max.
---@param candidates games.OpponentChoice[]
---@param range games.OpponentRange
---@param on_done fun(users: games.UserID[])
local function pick_opponents_from(candidates, range, on_done)
  ---@param picked games.UserID[]
  ---@param remaining games.OpponentChoice[]
  local function ask(picked, remaining)
    if #picked >= range.max or #remaining == 0 then
      on_done(picked)
      return
    end
    local can_finish = #picked >= range.min
    local so_far = table.concat(picked, ", ")
    local prompt = #picked == 0 and "Challenge whom?"
      or can_finish and ("Add another opponent? (%s)"):format(so_far)
      or ("Challenge whom else? (%s)"):format(so_far)
    picker.pick(can_finish and vim.list_extend({ DONE_PICKING }, remaining) or remaining, {
      prompt = prompt,
      format_item = function(choice)
        return choice.label
      end,
    }, function(choice)
      if not choice.user then
        on_done(picked)
        return
      end
      ask(
        vim.list_extend(vim.list_slice(picked), { choice.user }),
        vim.tbl_filter(function(other)
          return other ~= choice
        end, remaining)
      )
    end)
  end
  ask({}, candidates)
end

---@param client games.Client
---@param me games.UserID
---@param range games.OpponentRange
---@param on_done fun(users: games.UserID[])
local function pick_opponents(client, me, range, on_done)
  client:user_list(
    nil,
    on_ok(
      ---@param result games.UserListResult
      function(result)
        local others = sources.other_players(result.users, me)
        if #others == 0 then
          say("no other users")
          return
        end
        pick_opponents_from(vim.tbl_map(opponent_choice, others), range, on_done)
      end
    )
  )
end

---Opponents game_type takes or, when nil, the widest span over every game that takes any.
---@param games games.GameInfo[]
---@param game_type games.GameType|nil
---@return games.OpponentRange|nil nil when no such game takes an opponent
local function opponent_range(games, game_type)
  local range
  for _, info in ipairs(games) do
    if not players.is_solo_game(info) and (not game_type or info.id == game_type) then
      local min, max = math.max(info.players.min, 2) - 1, info.players.max - 1
      range = range and { min = math.min(range.min, min), max = math.max(range.max, max) }
        or { min = min, max = max }
    end
  end
  return range
end

---Asks which of infos to play, listed by name.
---@param infos games.GameInfo[]
---@param prompt string
---@param on_info fun(info: games.GameInfo)
local function pick_info(infos, prompt, on_info)
  picker.pick(infos, {
    prompt = prompt,
    format_item = function(info)
      return info.name
    end,
  }, on_info)
end

---@param info games.GameInfo
---@param players integer
---@return boolean
local function takes_players(info, players)
  return info.players.min <= players and players <= info.players.max
end

---@param client games.Client holding the catalog
---@param game_type games.GameType|nil picked interactively when nil
---@param players integer the challenger plus every opponent
---@param on_game fun(info: games.GameInfo)
local function pick_game(client, game_type, players, on_game)
  if not game_type then
    local playable = vim.tbl_filter(function(info)
      return takes_players(info, players)
    end, client:catalog_games())
    if #playable == 0 then
      say(("no %d-player games available"):format(players), ERROR)
      return
    end
    pick_info(playable, "Game:", on_game)
    return
  end
  local info = client:game_info(game_type)
  if not (info and takes_players(info, players)) then
    say(("no %d-player game %q"):format(players, game_type), ERROR)
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

---Why value does not fit the text option name, as the server would reject it, or nil when it fits.
---The server also rejects format characters (Cf), which it reports itself.
---@param name string
---@param option games.GameOption of type text
---@param value string
---@return string|nil
local function text_option_problem(name, option, value)
  if option.max_len and #value > option.max_len then
    return ("%s takes at most %d bytes, not %d"):format(text.humanize(name), option.max_len, #value)
  end
  if predicates.has_control_character(value) then
    return ("%s cannot hold control characters"):format(text.humanize(name))
  end
end

---Asks for the text option name until it fits, starting from typed; cancelling answers nothing.
---@param name string
---@param option games.GameOption of type text
---@param typed string
---@param on_value fun(value: string)
local function input_text_option(name, option, typed, on_value)
  local limit = option.max_len and (" (up to %d bytes)"):format(option.max_len) or ""
  vim.ui.input({ prompt = text.humanize(name) .. limit .. ": ", default = typed }, function(value)
    if value == nil then
      return
    end
    local problem = text_option_problem(name, option, value)
    if problem then
      say(problem, ERROR)
      input_text_option(name, option, value, on_value)
      return
    end
    on_value(value)
  end)
end

---Asks for each option in name order; on_done gets every option once all are answered, and is
---never called when one is cancelled.
---@param info games.GameInfo
---@param on_done fun(options: games.Options)
local function pick_options(info, on_done)
  local names = tables.sorted_keys(info.options)
  ---@param index integer
  ---@param chosen games.Options
  local function ask(index, chosen)
    local name = names[index]
    if not name then
      on_done(chosen)
      return
    end
    local option = info.options[name]
    ---@param value string
    local function answer(value)
      ask(index + 1, vim.tbl_extend("force", chosen, { [name] = value }))
    end
    if option.type == OptionType.TEXT then
      input_text_option(name, option, option.default, answer)
      return
    end
    picker.pick(default_first(option), {
      prompt = text.humanize(name) .. ":",
      format_item = function(value)
        return value == option.default and (value .. " (default)") or value
      end,
    }, answer)
  end
  ask(1, {})
end

---@param client games.Client
---@param info games.GameInfo
---@param opponents games.UserID[]
local function send_challenge(client, info, opponents)
  pick_options(info, function(options)
    client:challenge_create(
      { to = opponents, game = info.id, options = options },
      on_ok(function()
        say(
          ("challenged %s: %s"):format(
            table.concat(opponents, ", "),
            summary.game_summary(info.id, info, options)
          )
        )
      end)
    )
  end)
end

---Challenges invitees to game_type, asking for whatever is not given: opponents, game, options.
---@param client games.Client
---@param me games.UserID
---@param game_type games.GameType|nil
---@param invitees games.UserID[]
function M.challenge(client, me, game_type, invitees)
  ---@param catalog games.CatalogResult
  local function challenge_from_catalog(catalog)
    ---@param opponents games.UserID[]
    local function challenge_opponents(opponents)
      pick_game(client, game_type, #opponents + 1, function(info)
        send_challenge(client, info, opponents)
      end)
    end
    if #invitees > 0 then
      challenge_opponents(invitees)
      return
    end
    local range = opponent_range(catalog.games, game_type)
    if not range then
      say(
        game_type and ("no game %q to challenge anyone to"):format(game_type)
          or "no games available",
        ERROR
      )
      return
    end
    pick_opponents(client, me, range, challenge_opponents)
  end
  client:catalog_list(nil, on_ok(challenge_from_catalog))
end

---@type games.SubcommandRun
local function challenge(client, me, args)
  M.challenge(client, me, args[1], vim.list_slice(args, 2))
end

---Starts the solo game info, once its options are answered, and opens its board.
---@param client games.Client
---@param info games.GameInfo
local function start_solo(client, info)
  pick_options(info, function(options)
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
    M.challenge(client, me, info.id, {})
    return
  end
  start_solo(client, info)
end

---Starts the solo game named by args[1], or one picked from the catalogue's solo games.
---@type games.SubcommandRun
local function solo(client, _, args)
  local game_type = args[1]
  client:catalog_list(
    nil,
    on_ok(function()
      if not game_type then
        local solo_games = vim.tbl_filter(players.can_start_solo, client:catalog_games())
        if #solo_games == 0 then
          say("no solo games available", ERROR)
          return
        end
        pick_info(solo_games, "Solo game:", function(info)
          start_solo(client, info)
        end)
        return
      end
      local info = client:game_info(game_type)
      if not info then
        say(("no game %q"):format(game_type), ERROR)
        return
      end
      if not players.can_start_solo(info) then
        say(
          ("%s is not a solo game; challenge someone with :Games challenge %s"):format(
            game_type,
            game_type
          ),
          ERROR
        )
        return
      end
      start_solo(client, info)
    end)
  )
end

---The solo games of the catalogue the plugin's client has fetched, for completion.
---@return games.GameType[]
local function solo_game_types()
  local client = require("games").client
  local solo_games = client and vim.tbl_filter(players.can_start_solo, client:catalog_games()) or {}
  return vim.tbl_map(function(info)
    return info.id
  end, solo_games)
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
    action.send(client, { challenge_id = picked.challenge_id }, on_ok())
  end)
end

---Sends the action labelled label that applies to challenge, as me sees it.
---@param client games.Client
---@param me games.UserID
---@param challenge games.Challenge
---@param label string
function M.respond(client, me, challenge, label)
  local actions = challenge.from == me and OUTGOING_ACTIONS or INCOMING_ACTIONS
  local action = vim.iter(actions):find(function(candidate)
    return candidate.label == label
  end)
  if not action then
    say(("cannot %s this challenge"):format(label), ERROR)
    return
  end
  action.send(client, { challenge_id = challenge.challenge_id }, on_ok())
end

---@type games.SubcommandRun
local function challenges(client, me)
  client:challenge_list(
    nil,
    on_ok(function()
      local pending = client:pending_challenges()
      if #pending == 0 then
        say("no pending challenges")
        return
      end
      table.sort(pending, sources.incoming_first(me))
      client:game_infos(function(infos)
        picker.pick(pending, {
          prompt = "Pending challenges:",
          format_item = function(item)
            return summary.format_challenge(item, me, infos[item.game])
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
    say(("list takes %s, not %q"):format(table.concat(GAME_STATUSES, " or "), status), ERROR)
    return
  end
  client:game_list(
    { status = status },
    on_ok(
      ---@param result games.GameListResult
      function(result)
        if #result.games == 0 then
          say(("no %s games"):format(status))
          return
        end
        client:game_infos(function(infos)
          local now_ms = timer.unix_ms()
          picker.pick(result.games, {
            prompt = ("%s games:"):format(status),
            format_item = function(update)
              return summary.format_game(update, me, infos[update.game], now_ms)
            end,
          }, function(update)
            shell.open(client, update.game_id)
          end)
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
local function show_stats(client, me, args)
  M.show_stats(client, args[1] or me)
end

---@type games.SubcommandRun
local function show_leaderboard(client, _, args)
  M.show_leaderboard(client, args[1])
end

---@param tally games.ArchiveTally
---@return string
local function export_summary(tally)
  local Outcome = Archive.Outcome
  local parts = {
    ("%d written"):format(tally[Outcome.WRITTEN] or 0),
    ("%d already present"):format(tally[Outcome.ALREADY_PRESENT] or 0),
  }
  local failed = (tally[Outcome.REFUSED] or 0) + (tally[Outcome.FAILED] or 0)
  if failed > 0 then
    table.insert(parts, ("%d failed"):format(failed))
  end
  return "export: " .. table.concat(parts, ", ")
end

---@type games.SubcommandRun
local function export(client)
  local archive = require("games").archive
  if not archive then
    say("export needs archive.dir in setup()", ERROR)
    return
  end
  client:finished_games(function(finished)
    archive:backfill(finished, function(tally)
      say(export_summary(tally))
    end)
  end)
end

---@type games.SubcommandRun
local function next_game(client)
  shell.open_next(client)
end

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

---Opens the finder over source name once what it lists is fetched.
---@param client games.Client
---@param me games.UserID
---@param name games.SourceName
function M.find(client, me, name)
  local fetch = EXTRA_FETCHES[name] or function(_, done)
    done({})
  end
  fetch(client, function(extra)
    local picker = assert(require("games").config).picker
    finder.open(sources.build(name, sources.snapshot(client, me, extra)), picker)
  end)
end

---@type games.SubcommandRun
local function find(client, me, args)
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

---@type games.SubcommandRun
local function open_dashboard(client)
  dashboard.open(client)
end

---@return integer
local function puzzle_max_entries()
  return assert(require("games").config).puzzles.max_entries
end

---Imports args[1] into the puzzle set args[2], or one named after the file, keeping at most
---args[3] entries, or the configured max_entries.
---@param args string[]
local function import(args)
  local path, name, max = args[1], args[2], args[3]
  if not path then
    say("import takes a file: :Games import <file.csv|file.pgn> [set] [max]", ERROR)
    return
  end
  local max_entries = max == nil and puzzle_max_entries() or tonumber(max)
  if not predicates.is_positive_integer(max_entries) then
    say(("import: max %q is not a positive whole number"):format(max), ERROR)
    return
  end
  local opts = {
    name = name,
    max_entries = max_entries,
    on_progress = function(lines)
      say(("import: read %d lines of %s"):format(lines, path))
    end,
  }
  puzzles.import(vim.fn.expand(path), opts, function(err, result)
    if err then
      say("import: " .. err, ERROR)
      return
    end
    ---@cast result games.PuzzleImport
    say(puzzles.import_summary(result), #result.failures > 0 and vim.log.levels.WARN or nil)
  end)
end

---Starts the next entry of the puzzle set name, or of the latest imported one, then moves the set
---past it; an entry the server refuses is reported and skipped.
---@param client games.Client
---@param name string|nil
function M.start_puzzle(client, name)
  puzzles.load(name, function(err, set_name, set)
    if err then
      say(err, ERROR)
      return
    end
    ---@cast set_name string
    ---@cast set games.PuzzleSet
    local entry = set.entries[set.next]
    if not entry then
      say(
        ("puzzle set %s is done, all %d played; restart it from :Games find puzzles"):format(
          set_name,
          #set.entries
        )
      )
      return
    end
    local label = ("%s %d/%d (%s)"):format(set_name, set.next, #set.entries, entry.id)
    client:game_start(
      { game = PUZZLE_GAME, options = puzzles.options(entry) },
      ---@param update games.GameUpdate|nil
      function(start_err, update)
        if start_err and start_err.code ~= types.ErrorCode.BAD_REQUEST then
          say(start_err.message, ERROR)
          return
        end
        if not (start_err or update) then
          return
        end
        local save_err = puzzles.advance(set_name, entry)
        if save_err then
          say(("puzzle set %s: %s"):format(set_name, save_err), ERROR)
        end
        if start_err then
          say(("skipped %s: %s"):format(label, start_err.message), ERROR)
          return
        end
        ---@cast update games.GameUpdate
        say("puzzle " .. label)
        shell.open(client, update.game_id)
      end
    )
  end)
end

---Sends the puzzle set name back to its first entry once confirmed.
---@param name string
function M.restart_puzzles(name)
  if not prompt.confirm(("Restart puzzle set %s from its first entry?"):format(name)) then
    return
  end
  local err = puzzles.restart(name)
  if err then
    say(("puzzle set %s: %s"):format(name, err), ERROR)
    return
  end
  say(("puzzle set %s starts again from its first entry"):format(name))
end

---@type games.SubcommandRun
local function puzzle(client, _, args)
  M.start_puzzle(client, args[1])
end

---@param arg_lead string
---@return string[]
local function file_choices(arg_lead)
  return vim.fn.getcompletion(arg_lead, "file")
end

---@param choices string[]
---@return fun(): string[]
local function always(choices)
  return function()
    return choices
  end
end

---@type games.Subcommand[]
local SUBCOMMANDS = {
  { name = "challenge", usage = "challenge [game] [user…]", run = challenge },
  { name = "challenges", usage = "challenges", run = challenges },
  { name = "solo", usage = "solo [game]", run = solo, arg_choices = solo_game_types },
  {
    name = "list",
    usage = "list [ongoing|finished]",
    run = list,
    arg_choices = always(GAME_STATUSES),
  },
  { name = "stats", usage = "stats [user]", run = show_stats },
  { name = "leaderboard", usage = "leaderboard [game]", run = show_leaderboard },
  { name = "next", usage = "next", run = next_game },
  { name = "export", usage = "export", run = export },
  { name = "find", usage = "find [source]", run = find, arg_choices = always(sources.NAMES) },
  {
    name = "import",
    usage = "import <file> [set] [max]",
    run_offline = import,
    arg_choices = file_choices,
  },
  { name = "puzzle", usage = "puzzle [set]", run = puzzle, arg_choices = puzzles.set_names },
}

---@param name string
---@return games.Subcommand|nil
local function find_subcommand(name)
  return vim.iter(SUBCOMMANDS):find(function(subcommand)
    return subcommand.name == name
  end)
end

---@param client games.Client
---@param fargs string[] as typed, for the label a pending run is reported by
---@param run games.SubcommandRun
---@param args string[]
local function run_connected(client, fargs, run, args)
  local label = table.concat(vim.list_extend({ ":Games" }, fargs), " ")
  login.ensure_connected(client, label, function()
    local me = notify.require_me(client)
    if me then
      run(client, me, args)
    end
  end)
end

---Runs `:Games <fargs>`; with no subcommand it opens the dashboard.
---@param fargs string[]
function M.run(fargs)
  local client = require("games").client
  if not client then
    say('call require("games").setup() first', ERROR)
    return
  end
  local name = fargs[1]
  if not name then
    run_connected(client, fargs, open_dashboard, {})
    return
  end
  local subcommand = find_subcommand(name)
  if not subcommand then
    say(("unknown subcommand %q"):format(name), ERROR)
    return
  end
  local args = vim.list_slice(fargs, 2)
  if subcommand.run_offline then
    subcommand.run_offline(args)
    return
  end
  run_connected(client, fargs, assert(subcommand.run), args)
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
    candidates = subcommand and subcommand.arg_choices and subcommand.arg_choices(arg_lead) or {}
  end
  return vim.tbl_filter(function(candidate)
    return vim.startswith(candidate, arg_lead)
  end, candidates)
end

return M
