local shell = require("games.ui.shell")
local text = require("games.ui.text")

local TIME_CONTROL_OPTION = "time_control"
local YOUR_TURN_MARKER = "●"
local GAME_WIDTH = 7
local OPPONENTS_WIDTH = 8
local TIME_CONTROL_WIDTH = 6
local TURN_WIDTH = 11
local SHORT_ID_LENGTH = 4

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
local function turn_text(update, me)
  if update.result then
    return "finished"
  end
  return vim.tbl_contains(update.to_act, me) and "your move" or "waiting"
end

---@param update games.GameUpdate
---@param me games.UserID
---@return string
local function clock(update, me)
  if not (update.clock and update.clock.remaining_ms[me]) then
    return ""
  end
  return shell.clock_text(update, me, 0)
end

---One searchable picker line, e.g. `● chess  vs ravi    10+5  your move  4:51  last: Nf3  #7f3a`.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.format_game(update, me)
  local my_turn = update.result == nil and vim.tbl_contains(update.to_act, me)
  local last = shell.describe(update)
  return table.concat({
    (my_turn and YOUR_TURN_MARKER or " ") .. " ",
    column(update.game, GAME_WIDTH),
    "vs " .. column(table.concat(shell.opponents(update, me), ","), OPPONENTS_WIDTH),
    column(update.options and update.options[TIME_CONTROL_OPTION] or "", TIME_CONTROL_WIDTH),
    column(turn_text(update, me), TURN_WIDTH),
    clock(update, me),
    "  last: " .. (last ~= "" and last or "-"),
    "  #" .. update.game_id:sub(1, SHORT_ID_LENGTH),
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
