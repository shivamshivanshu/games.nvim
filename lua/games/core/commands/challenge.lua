local notify = require("games.core.notify")
local picker = require("games.ui.picker")
local players = require("games.core.players")
local predicates = require("games.core.predicates")
local reply = require("games.core.commands.reply")
local summary = require("games.core.summary")
local tables = require("games.core.tables")
local text = require("games.ui.text")
local types = require("games.types")

local OptionType = types.OptionType
local ERROR = vim.log.levels.ERROR

local M = {}

local say = notify.say
local on_ok = reply.on_ok

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
        local others = players.other_players(result.users, me)
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
function M.pick_info(infos, prompt, on_info)
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
    M.pick_info(playable, "Game:", on_game)
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
function M.pick_options(info, on_done)
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
  M.pick_options(info, function(options)
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
function M.run(client, me, args)
  M.challenge(client, me, args[1], vim.list_slice(args, 2))
end

return M
