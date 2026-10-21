local compose = require("games.ui.compose")
local controls = require("games.core.controls")
local describe = require("games.core.describe")
local kv = require("games.ui.widgets.kv")
local picker = require("games.ui.picker")
local players = require("games.core.players")
local stats = require("games.core.stats")
local tables = require("games.core.tables")
local text = require("games.ui.text")
local timer = require("games.core.timer")
local types = require("games.types")

local NONE = "-"
local START_DATE_FORMAT = "%Y-%m-%d"

---@enum games.SourceName
local SourceName = {
  REQUESTS = "requests",
  GAMES = "games",
  TYPES = "types",
  PLAYERS = "players",
  PUZZLES = "puzzles",
}

---@enum games.SourceActionName
local ActionName = {
  ACCEPT = "accept",
  DECLINE = "decline",
  CANCEL = "cancel",
  OPEN = "open",
  RESIGN = "resign",
  GIVE_UP = "give up",
  START = "start",
  LEADERBOARD = "leaderboard",
  CHALLENGE = "challenge",
  STATS = "stats",
  RESTART = "restart",
}

---@class games.SourceAction
---@field name games.SourceActionName
---@field run fun(item: any)
---@field applies? fun(item: any): boolean whether it acts on item; default always

---What a finder or the dashboard lists, and what it can do with each item.
---@class games.Source
---@field name games.SourceName
---@field title string
---@field items any[] in display order
---@field key fun(item: any): string an id that stays the item's across rebuilds
---@field format fun(item: any): string
---@field preview fun(item: any): string[]|nil
---@field actions games.SourceAction[] in the order a choice offers them
---@field default_action? games.SourceActionName run on confirm; nil asks which action to run

---Everything a source is built from, read once so every source of one build agrees.
---@class games.SourceSnapshot
---@field client games.Client actions send through it
---@field me games.UserID
---@field now_ms integer unix ms the clocks are shown at
---@field games games.GameUpdate[]
---@field challenges games.Challenge[] pending
---@field catalog games.GameInfo[]
---@field infos table<games.GameType, games.GameInfo> the catalog by game
---@field users games.UserPresence[]
---@field stats? games.StatsResult over every user, as stats.get without user returns it
---@field puzzle_sets games.PuzzleSetSummary[]

---What the client cache does not hold, fetched by whoever builds the snapshot.
---@class games.SourceExtra
---@field finished? games.GameUpdate[] as game.list finished returns them
---@field users? games.UserPresence[]
---@field stats? games.StatsResult
---@field puzzle_sets? games.PuzzleSetSummary[]

local M = {}

M.SourceName = SourceName
M.ActionName = ActionName
---@type games.SourceName[] in the order completion and a source choice offer them
M.NAMES = {
  SourceName.REQUESTS,
  SourceName.GAMES,
  SourceName.TYPES,
  SourceName.PLAYERS,
  SourceName.PUZZLES,
}

---Required on first use, because commands opens the finder over these sources.
local function commands()
  return require("games.core.commands")
end

---@param pairs games.KvPair[]
---@return string[]
local function kv_lines(pairs)
  return compose.render(kv({ pairs = pairs })).lines
end

---@param values string[]
---@return string
local function listed(values)
  return #values > 0 and table.concat(values, ", ") or NONE
end

---Incoming first, then the soonest to expire.
---@param me games.UserID
---@return fun(a: games.Challenge, b: games.Challenge): boolean
function M.incoming_first(me)
  return function(a, b)
    local a_incoming, b_incoming = a.from ~= me, b.from ~= me
    if a_incoming ~= b_incoming then
      return a_incoming
    end
    if a.expires_at ~= b.expires_at then
      return a.expires_at < b.expires_at
    end
    return a.challenge_id < b.challenge_id
  end
end

---@param a games.UserPresence
---@param b games.UserPresence
---@return boolean
function M.online_first(a, b)
  if a.online ~= b.online then
    return a.online
  end
  return a.user < b.user
end

---@enum (private) games.GameStage
local GameStage = { YOUR_MOVE = 1, WAITING = 2, FINISHED = 3 }

---@param update games.GameUpdate
---@param me games.UserID
---@return games.GameStage
local function stage(update, me)
  if update.result then
    return GameStage.FINISHED
  end
  return players.is_to_act(update, me) and GameStage.YOUR_MOVE or GameStage.WAITING
end

---@param update games.GameUpdate
---@return integer
local function recency(update)
  return update.finished_at or update.started_at or 0
end

---Your move first, then the games waiting on others, then finished ones; newest first in each.
---@param me games.UserID
---@return fun(a: games.GameUpdate, b: games.GameUpdate): boolean
local function by_urgency(me)
  return function(a, b)
    local a_stage, b_stage = stage(a, me), stage(b, me)
    if a_stage ~= b_stage then
      return a_stage < b_stage
    end
    if recency(a) ~= recency(b) then
      return recency(a) > recency(b)
    end
    return a.game_id < b.game_id
  end
end

---Whether update is ongoing and waiting on me.
---@param update games.GameUpdate
---@param me games.UserID
---@return boolean
function M.is_your_move(update, me)
  return stage(update, me) == GameStage.YOUR_MOVE
end

---The cache's games with the listed ones it lacks; of two copies of a game the later seq wins.
---@param cached games.GameUpdate[]
---@param listed games.GameUpdate[]
---@return games.GameUpdate[]
local function merged(cached, listed)
  local by_id = {}
  for _, update in ipairs(vim.list_extend(vim.list_slice(listed), cached)) do
    local held = by_id[update.game_id]
    if not held or update.seq > held.seq then
      by_id[update.game_id] = update
    end
  end
  return vim.tbl_values(by_id)
end

---Reads what client holds now, with extra on top.
---@param client games.Client
---@param me games.UserID
---@param extra? games.SourceExtra
---@return games.SourceSnapshot
function M.snapshot(client, me, extra)
  extra = extra or {}
  return {
    client = client,
    me = me,
    now_ms = timer.unix_ms(),
    games = merged(client:games(), extra.finished or {}),
    challenges = client:pending_challenges(),
    catalog = client:catalog_games(),
    infos = client:catalog_infos(),
    users = extra.users or {},
    stats = extra.stats,
    puzzle_sets = extra.puzzle_sets or {},
  }
end

---@param challenge games.Challenge
---@param me games.UserID
---@return boolean
local function is_incoming(challenge, me)
  return challenge.from ~= me
end

---@param options games.Options
---@return string
local function options_text(options)
  local names = tables.sorted_keys(options)
  return listed(vim.tbl_map(function(name)
    return ("%s=%s"):format(name, options[name])
  end, names))
end

---@param snapshot games.SourceSnapshot
---@return games.Source
local function requests(snapshot)
  local me, client = snapshot.me, snapshot.client
  local infos = snapshot.infos
  local pending = vim.list_slice(snapshot.challenges)
  table.sort(pending, M.incoming_first(me))
  ---@param name games.SourceActionName
  ---@param incoming boolean whether it answers a challenge to me, or withdraws mine
  ---@return games.SourceAction
  local function respond(name, incoming)
    return {
      name = name,
      applies = function(challenge)
        return is_incoming(challenge, me) == incoming
      end,
      run = function(challenge)
        commands().respond(client, me, challenge, name)
      end,
    }
  end
  return {
    name = SourceName.REQUESTS,
    title = "Challenges",
    items = pending,
    key = function(challenge)
      return challenge.challenge_id
    end,
    format = function(challenge)
      return picker.format_challenge(challenge, me, infos[challenge.game])
    end,
    preview = function(challenge)
      return kv_lines({
        { "from", challenge.from },
        { "to", table.concat(challenge.to, ", ") },
        { "accepted", listed(challenge.accepted) },
        { "game", picker.game_summary(challenge.game, infos[challenge.game], challenge.options) },
        { "options", options_text(challenge.options) },
        {
          "expires",
          "in " .. describe.clock_text(math.max(0, challenge.expires_at - snapshot.now_ms)),
        },
      })
    end,
    actions = {
      respond(ActionName.ACCEPT, true),
      respond(ActionName.DECLINE, true),
      respond(ActionName.CANCEL, false),
    },
  }
end

---@param update games.GameUpdate
---@param now_ms integer
---@return string
local function clocks_text(update, now_ms)
  local elapsed_ms = describe.clock_elapsed_ms(update, now_ms)
  return table.concat(
    vim.tbl_map(function(player)
      local remaining_ms = describe.clock_remaining_ms(update, player.user, elapsed_ms)
      return ("%s %s"):format(player.user, describe.clock_text(remaining_ms))
    end, update.players),
    ", "
  )
end

---@param update games.GameUpdate
---@param me games.UserID
---@param info? games.GameInfo
---@param now_ms integer
---@return string[]
local function game_preview(update, me, info, now_ms)
  local seats = vim.tbl_map(function(player)
    return ("%s (%s)"):format(player.user, player.seat)
  end, update.players)
  local last = describe.events(update)
  local rows = {
    { "game", info and info.name or update.game },
    { "players", table.concat(seats, ", ") },
    {
      "status",
      update.result and describe.result_text(update.result) or describe.turn_text(update, me),
    },
  }
  if update.clock then
    rows[#rows + 1] = { "clock", clocks_text(update, now_ms) }
  end
  vim.list_extend(rows, {
    { "last", last ~= "" and last or NONE },
    { "started", update.started_at and describe.date(update, START_DATE_FORMAT) or NONE },
    { "id", update.game_id },
  })
  return kv_lines(rows)
end

---Resigning, named for whether the game is played alone, so a solo game offers to give up.
---@param name games.SourceActionName
---@param client games.Client
---@param solo boolean which games it acts on: those played alone, or the others
---@return games.SourceAction
local function resign_action(name, client, solo)
  return {
    name = name,
    applies = function(update)
      return update.result == nil
        and players.is_solo(update) == solo
        and vim.tbl_contains(update.controls, types.Control.RESIGN)
    end,
    run = function(update)
      controls.send(
        client,
        update,
        types.Control.RESIGN,
        describe.resign_wording(update),
        controls.report_message
      )
    end,
  }
end

---@param snapshot games.SourceSnapshot
---@return games.Source
local function games(snapshot)
  local me, client = snapshot.me, snapshot.client
  local infos = snapshot.infos
  local updates = vim.list_slice(snapshot.games)
  table.sort(updates, by_urgency(me))
  return {
    name = SourceName.GAMES,
    title = "Games",
    items = updates,
    key = function(update)
      return update.game_id
    end,
    format = function(update)
      return picker.format_game(update, me, infos[update.game], snapshot.now_ms)
    end,
    preview = function(update)
      return game_preview(update, me, infos[update.game], snapshot.now_ms)
    end,
    actions = {
      {
        name = ActionName.OPEN,
        run = function(update)
          require("games.ui.shell").open(client, update.game_id)
        end,
      },
      resign_action(ActionName.RESIGN, client, false),
      resign_action(ActionName.GIVE_UP, client, true),
    },
    default_action = ActionName.OPEN,
  }
end

---@param info games.GameInfo
---@return string
local function players_text(info)
  local range = info.players
  if players.is_solo_game(info) then
    return describe.SOLO
  end
  if range.min == range.max then
    return ("%d players"):format(range.max)
  end
  return ("%d-%d players"):format(range.min, range.max)
end

---@param option games.GameOption
---@return string
local function option_text(option)
  if option.type == types.OptionType.TEXT then
    return option.max_len and ("text, up to %d bytes"):format(option.max_len) or "text"
  end
  return ("%s (default %s)"):format(table.concat(option.values, ", "), option.default)
end

---@param info games.GameInfo
---@return string[]
local function type_preview(info)
  local rows = {
    { "game", info.name },
    { "players", players_text(info) },
    { "controls", listed(info.capabilities) },
  }
  for _, name in ipairs(tables.sorted_keys(info.options)) do
    rows[#rows + 1] = { text.humanize(name), option_text(info.options[name]) }
  end
  for _, spec in ipairs(info.actions or {}) do
    rows[#rows + 1] = { spec.kind, spec.description }
  end
  return kv_lines(rows)
end

---@param snapshot games.SourceSnapshot
---@return games.Source
local function game_types(snapshot)
  local me, client = snapshot.me, snapshot.client
  local infos = vim.list_slice(snapshot.catalog)
  table.sort(infos, function(a, b)
    return a.name < b.name
  end)
  local name_width = text.max_display_width(vim.tbl_map(function(info)
    return info.name
  end, infos))
  return {
    name = SourceName.TYPES,
    title = "New game",
    items = infos,
    key = function(info)
      return info.id
    end,
    format = function(info)
      return ("%s  %s"):format(text.pad_to_width(info.name, name_width), players_text(info))
    end,
    preview = type_preview,
    actions = {
      {
        name = ActionName.START,
        run = function(info)
          commands().start(client, me, info)
        end,
      },
      {
        name = ActionName.LEADERBOARD,
        run = function(info)
          commands().show_leaderboard(client, info.id)
        end,
      },
    },
    default_action = ActionName.START,
  }
end

---@param user games.UserID
---@param result games.StatsResult
---@return games.KvPair[]
local function player_stats_rows(user, result)
  local rank = vim.iter(ipairs(result.leaderboard)):find(function(_, entry)
    return entry.user == user
  end)
  ---@generic T: {user: games.UserID}
  ---@param records T[]
  ---@return T[]
  local function own(records)
    return vim.tbl_filter(function(record)
      return record.user == user
    end, records)
  end
  local rows = {
    { "rank", rank and ("%d of %d"):format(rank, #result.leaderboard) or NONE },
  }
  return vim.list_extend(
    rows,
    stats.stats_pairs({
      records = own(result.records),
      head_to_head = own(result.head_to_head),
      leaderboard = {},
    })
  )
end

---@param snapshot games.SourceSnapshot
---@return games.Source
local function player_list(snapshot)
  local me, client = snapshot.me, snapshot.client
  local others = vim.tbl_filter(function(presence)
    return presence.user ~= me
  end, snapshot.users)
  table.sort(others, M.online_first)
  return {
    name = SourceName.PLAYERS,
    title = "Players",
    items = others,
    key = function(presence)
      return presence.user
    end,
    format = function(presence)
      return presence.online and (presence.user .. " (online)") or presence.user
    end,
    preview = function(presence)
      local rows = { { "user", presence.user }, { "online", presence.online and "yes" or "no" } }
      if snapshot.stats then
        vim.list_extend(rows, player_stats_rows(presence.user, snapshot.stats))
      end
      return kv_lines(rows)
    end,
    actions = {
      {
        name = ActionName.CHALLENGE,
        run = function(presence)
          commands().challenge(client, me, nil, { presence.user })
        end,
      },
      {
        name = ActionName.STATS,
        run = function(presence)
          commands().show_stats(client, presence.user)
        end,
      },
    },
    default_action = ActionName.CHALLENGE,
  }
end

---@param set games.PuzzleSetSummary
---@return string
local function puzzle_progress(set)
  if set.next > set.entries then
    return ("all %d played"):format(set.entries)
  end
  return ("next %d of %d"):format(set.next, set.entries)
end

---@param snapshot games.SourceSnapshot
---@return games.Source
local function puzzle_sets(snapshot)
  local client = snapshot.client
  local name_width = text.max_display_width(vim.tbl_map(function(set)
    return set.name
  end, snapshot.puzzle_sets))
  return {
    name = SourceName.PUZZLES,
    title = "Puzzles",
    items = snapshot.puzzle_sets,
    key = function(set)
      return set.name
    end,
    format = function(set)
      return ("%s  %s"):format(text.pad_to_width(set.name, name_width), puzzle_progress(set))
    end,
    preview = function(set)
      return kv_lines({
        { "set", set.name },
        { "entries", tostring(set.entries) },
        { "progress", puzzle_progress(set) },
      })
    end,
    actions = {
      {
        name = ActionName.START,
        run = function(set)
          commands().start_puzzle(client, set.name)
        end,
      },
      {
        name = ActionName.RESTART,
        run = function(set)
          commands().restart_puzzles(set.name)
        end,
      },
    },
    default_action = ActionName.START,
  }
end

---@type table<games.SourceName, fun(snapshot: games.SourceSnapshot): games.Source>
local BUILDERS = {
  [SourceName.REQUESTS] = requests,
  [SourceName.GAMES] = games,
  [SourceName.TYPES] = game_types,
  [SourceName.PLAYERS] = player_list,
  [SourceName.PUZZLES] = puzzle_sets,
}

---@param name string
---@return boolean
function M.is_source_name(name)
  return BUILDERS[name] ~= nil
end

---@param name games.SourceName
---@param snapshot games.SourceSnapshot
---@return games.Source
function M.build(name, snapshot)
  return BUILDERS[name](snapshot)
end

---The actions of source that act on item, in source order.
---@param source games.Source
---@param item any
---@return games.SourceAction[]
function M.applicable(source, item)
  return vim.tbl_filter(function(action)
    return not action.applies or action.applies(item)
  end, source.actions)
end

---The first of names, in names order, whose action acts on item.
---@param source games.Source
---@param item any
---@param names games.SourceActionName[]
---@return games.SourceAction|nil
function M.find_action(source, item, names)
  local actions = M.applicable(source, item)
  for _, name in ipairs(names) do
    local action = vim.iter(actions):find(function(candidate)
      return candidate.name == name
    end)
    if action then
      return action
    end
  end
end

return M
