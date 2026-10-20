local describe = require("games.core.describe")
local players = require("games.core.players")
local text = require("games.ui.text")

local TIME_CONTROL_OPTION = "time_control"
local YOUR_TURN_MARKER = "●"
local MARKER_WIDTH = 2
local GAME_WIDTH = 7
local OPPONENTS_WIDTH = 8
local TIME_CONTROL_WIDTH = 6
local TURN_WIDTH = 10
local CLOCK_WIDTH = 5

local M = {}

---Pads str to width, keeping at least one blank so an overlong value never runs into the next column.
---@param str string
---@param width integer
---@return string
local function column(str, width)
  return text.pad_to_width(str, width - 1) .. " "
end

---@param update games.GameUpdate
---@param me games.UserID
---@return string
local function clock(update, me)
  if not (update.clock and update.clock.remaining_ms[me]) then
    return ""
  end
  return describe.clock_text(update, me, 0)
end

---One searchable picker line, e.g. `● chess  vs ravi    10+5  your move  4:51  last: Nf3  #7f3a`.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.format_game(update, me)
  local last = describe.events(update)
  return table.concat({
    text.pad_to_width(players.is_to_act(update, me) and YOUR_TURN_MARKER or "", MARKER_WIDTH),
    column(update.game, GAME_WIDTH),
    "vs " .. column(describe.opponent_names(update, me, ","), OPPONENTS_WIDTH),
    column(update.options and update.options[TIME_CONTROL_OPTION] or "", TIME_CONTROL_WIDTH),
    column(describe.turn_text(update, me), TURN_WIDTH),
    text.align_right(clock(update, me), CLOCK_WIDTH),
    "  last: " .. (last ~= "" and last or "-"),
    "  #" .. describe.short_id(update.game_id),
  })
end

---"<Game name> <time control>", e.g. "Chess 10+5".
---@param game_name string
---@param options? games.Options
---@return string
function M.game_summary(game_name, options)
  local time_control = options and options[TIME_CONTROL_OPTION]
  return time_control and ("%s %s"):format(game_name, time_control) or game_name
end

---Calls fn with each catalog game's display name; on a failed request the map is empty.
---@param client games.Client
---@param fn fun(names: table<games.GameType, string>)
function M.with_game_names(client, fn)
  client:catalog_list(nil, function(_, catalog)
    local names = {}
    for _, info in ipairs(catalog and catalog.games or {}) do
      names[info.id] = info.name
    end
    fn(names)
  end)
end

---@param challenge games.Challenge
---@param me games.UserID
---@param game_name string
---@return string
function M.format_challenge(challenge, me, game_name)
  local summary = M.game_summary(game_name, challenge.options)
  if challenge.from == me then
    return ("you → %s: %s"):format(table.concat(challenge.to, ", "), summary)
  end
  return ("%s → you: %s"):format(challenge.from, summary)
end

---vim.ui.select that calls on_pick only when the user picked something.
---@generic T
---@param items T[]
---@param opts vim.ui.select.Opts
---@param on_pick fun(item: T)
function M.pick(items, opts, on_pick)
  vim.ui.select(items, opts, function(item)
    if item ~= nil then
      on_pick(item)
    end
  end)
end

return M
