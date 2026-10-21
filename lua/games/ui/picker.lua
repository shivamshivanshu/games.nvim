local describe = require("games.core.describe")
local players = require("games.core.players")
local text = require("games.ui.text")
local types = require("games.types")

local YOUR_TURN_MARKER = "●"
local MARKER_WIDTH = 2
local GAME_WIDTH = 7
local VERSUS_WIDTH = 11
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
---@param now_ms integer unix ms
---@return string
local function clock(update, me, now_ms)
  if not (update.clock and update.clock.remaining_ms[me]) then
    return ""
  end
  local elapsed_ms = describe.clock_elapsed_ms(update, now_ms)
  return describe.clock_text(describe.clock_remaining_ms(update, me, elapsed_ms))
end

---The time control in options, found by the role the catalog gives its option.
---@param info? games.GameInfo nil when the catalog does not list the game
---@param options? games.Options
---@return string|nil
local function time_control(info, options)
  if not (info and options) then
    return nil
  end
  for name, option in pairs(info.options) do
    if option.role == types.OptionRole.CLOCK then
      return options[name]
    end
  end
end

---One searchable picker line, e.g. `● chess  vs ravi    10+5  your move  4:51  last: Nf3  #7f3a`;
---a game played alone shows `solo` in place of `vs ravi`.
---@param update games.GameUpdate
---@param me games.UserID
---@param info? games.GameInfo the game's catalog entry
---@param now_ms integer unix ms the clock is shown at
---@return string
function M.format_game(update, me, info, now_ms)
  local last = describe.events(update)
  return table.concat({
    text.pad_to_width(players.is_to_act(update, me) and YOUR_TURN_MARKER or "", MARKER_WIDTH),
    column(update.game, GAME_WIDTH),
    column(describe.versus_text(update, me, ","), VERSUS_WIDTH),
    column(time_control(info, update.options) or "", TIME_CONTROL_WIDTH),
    column(describe.turn_text(update, me), TURN_WIDTH),
    text.align_right(clock(update, me, now_ms), CLOCK_WIDTH),
    "  last: " .. (last ~= "" and last or "-"),
    "  #" .. describe.short_id(update.game_id),
  })
end

---"<Game name> <time control>", e.g. "Chess 10+5"; the game type stands in for an unlisted game.
---@param game games.GameType
---@param info? games.GameInfo the game's catalog entry
---@param options? games.Options
---@return string
function M.game_summary(game, info, options)
  local name = info and info.name or game
  local chosen = time_control(info, options)
  return chosen and ("%s %s"):format(name, chosen) or name
end

---@param challenge games.Challenge
---@param me games.UserID
---@param info? games.GameInfo the game's catalog entry
---@return string
function M.format_challenge(challenge, me, info)
  local summary = M.game_summary(challenge.game, info, challenge.options)
  if challenge.from == me then
    return ("you → %s: %s"):format(table.concat(challenge.to, ", "), summary)
  end
  return ("%s → you: %s"):format(challenge.from, summary)
end

---@param presence games.UserPresence
---@return string
function M.format_presence(presence)
  return presence.online and (presence.user .. " (online)") or presence.user
end

---vim.ui.select that calls on_pick only when the user picked something, and otherwise on_cancel.
---@generic T
---@param items T[]
---@param opts vim.ui.select.Opts
---@param on_pick fun(item: T)
---@param on_cancel? fun()
function M.pick(items, opts, on_pick, on_cancel)
  vim.ui.select(items, opts, function(item)
    if item ~= nil then
      on_pick(item)
    elseif on_cancel then
      on_cancel()
    end
  end)
end

return M
