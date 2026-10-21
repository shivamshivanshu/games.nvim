local cards = require("games.ui.cards")
local dice = require("games.ui.dice")
local players = require("games.core.players")
local registry = require("games.core.registry")
local text = require("games.ui.text")
local timer = require("games.core.timer")
local types = require("games.types")

local SHORT_ID_LENGTH = 4
local SECONDS_PER_MINUTE = 60

---How event text names seats and draws a game's pieces, built once per update.
---@class games.Talk
---@field name fun(seat: games.Seat): string
---@field art? games.ArtSet the game's asset pack in the user's glyph mode
---@field ambiwidth games.Ambiwidth
---@field cards fun(cards: games.CardKey[]): string the cards' glyphs, from art
---@field faces fun(faces: games.DieFace[]): string the dice's glyphs, from art

---@alias games.Describer fun(event: games.Event, talk: games.Talk): string|nil

local M = {}

---What stands in for the opponents of a game played alone.
M.SOLO = "solo"

---@param update games.GameUpdate
---@param me games.UserID
---@param separator string
---@return string
function M.opponent_names(update, me, separator)
  local opponents = vim
    .iter(update.players)
    :map(function(player)
      return player.user
    end)
    :filter(function(user)
      return user ~= me
    end)
    :totable()
  return table.concat(opponents, separator)
end

---The id prefix that buffer names and pickers show.
---@param game_id games.GameID
---@return string
function M.short_id(game_id)
  return game_id:sub(1, SHORT_ID_LENGTH)
end

---The opponents' names, or "solo" for a game played alone.
---@param update games.GameUpdate
---@param me games.UserID
---@param separator string
---@return string
function M.opponents_text(update, me, separator)
  return players.is_solo(update) and M.SOLO or M.opponent_names(update, me, separator)
end

---"vs <opponents>", or "solo" for a game played alone.
---@param update games.GameUpdate
---@param me games.UserID
---@param separator string
---@return string
function M.versus_text(update, me, separator)
  return players.is_solo(update) and M.SOLO or "vs " .. M.opponent_names(update, me, separator)
end

---"<game> vs <opponents>" or "<game> solo", the name notifications and errors use for a game.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.title(update, me)
  return ("%s %s"):format(update.game, M.versus_text(update, me, ", "))
end

---@class games.ControlWording
---@field label string
---@field confirm? string asked before the control is sent; nil sends it at once

---@class games.ConfirmedWording: games.ControlWording
---@field confirm string

---@type games.ConfirmedWording
local RESIGN_WORDING = { label = "resign", confirm = "Resign this game?" }
---@type games.ConfirmedWording
local GIVE_UP_WORDING = { label = "give up", confirm = "Give up this game?" }

---How the resign control reads: giving up, in a game played alone.
---@param update games.GameUpdate
---@return games.ConfirmedWording
function M.resign_wording(update)
  return players.is_solo(update) and GIVE_UP_WORDING or RESIGN_WORDING
end

---Time since the clock's reading, never negative so a client clock behind the server's charges none.
---@param update games.GameUpdate
---@param now_ms integer unix ms
---@return integer 0 without a clock
function M.clock_elapsed_ms(update, now_ms)
  if not update.clock then
    return 0
  end
  return math.max(0, now_ms - update.clock.as_of_ms)
end

---Whether update's game is ongoing with a clock counting down.
---@param update games.GameUpdate
---@return boolean
function M.has_running_clock(update)
  return update.result == nil and update.clock ~= nil and update.clock.running ~= nil
end

---@param update games.GameUpdate with a clock
---@param user games.UserID
---@param elapsed_ms integer charged to user's clock if it is the running one
---@return integer never negative
function M.clock_remaining_ms(update, user, elapsed_ms)
  local remaining_ms = update.clock.remaining_ms[user] or 0
  if update.clock.running == user then
    remaining_ms = remaining_ms - elapsed_ms
  end
  return math.max(0, remaining_ms)
end

---@class games.ClockReading
---@field remaining_ms integer never negative
---@field text string "m:ss"

---Each player's clock elapsed_ms after the clock's reading, in update.players order.
---@param update games.GameUpdate with a clock
---@param elapsed_ms integer charged to the running clock
---@return games.ClockReading[]
function M.clock_readings(update, elapsed_ms)
  return vim.tbl_map(function(player)
    local remaining_ms = M.clock_remaining_ms(update, player.user, elapsed_ms)
    return { remaining_ms = remaining_ms, text = M.clock_text(remaining_ms) }
  end, update.players)
end

---Each player's clock text at now_ms, by user.
---@param update games.GameUpdate with a clock
---@param now_ms integer unix ms
---@return table<games.UserID, string>
function M.clock_texts(update, now_ms)
  local readings = M.clock_readings(update, M.clock_elapsed_ms(update, now_ms))
  local texts = {}
  for i, player in ipairs(update.players) do
    texts[player.user] = readings[i].text
  end
  return texts
end

---"m:ss", rounding up to the whole second.
---@param remaining_ms integer never negative
---@return string
function M.clock_text(remaining_ms)
  local seconds = math.ceil(remaining_ms / timer.MS_PER_SECOND)
  return ("%d:%02d"):format(math.floor(seconds / SECONDS_PER_MINUTE), seconds % SECONDS_PER_MINUTE)
end

---The local day the game started on, in os.date format.
---@param update games.GameUpdate
---@param format string e.g. "%Y-%m-%d"
---@return string
function M.date(update, format)
  return os.date(format, math.floor(update.started_at / timer.MS_PER_SECOND)) --[[@as string]]
end

---@param result games.WireResult
---@return string
function M.result_text(result)
  local reason = text.humanize(result.reason)
  if result.kind == types.ResultKind.ABORTED then
    return "Result: aborted"
  end
  if result.kind == types.ResultKind.DRAW then
    return ("Result: draw (%s)"):format(reason)
  end
  if #result.winners == 0 then
    return ("Result: lost (%s)"):format(reason)
  end
  return ("Result: %s won (%s)"):format(table.concat(result.winners, ", "), reason)
end

---@alias games.Outcome "won"|"lost"|"drawn"|"aborted"

---How the result went for me.
---@param result games.WireResult
---@param me games.UserID
---@return games.Outcome
function M.outcome(result, me)
  if result.kind == types.ResultKind.ABORTED then
    return "aborted"
  end
  if result.kind == types.ResultKind.DRAW then
    return "drawn"
  end
  return vim.tbl_contains(result.winners, me) and "won" or "lost"
end

---@type table<games.GameStage, string>
local STAGE_TEXT = {
  [players.GameStage.YOUR_MOVE] = "your move",
  [players.GameStage.WAITING] = "waiting",
  [players.GameStage.FINISHED] = "finished",
}

---"your move", "waiting" or "finished", as me sees the game.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.turn_text(update, me)
  return STAGE_TEXT[players.stage(update, me)]
end

---@param event games.Event
---@param update games.GameUpdate
---@return string
local function fallback_description(event, update)
  local actor = event.by and players.name_of_seat(update, event.by)
  return actor and ("%s: %s"):format(actor, event.kind) or event.kind
end

---Required on use: assets reports through notify, which requires this module.
---@param game games.GameType
---@param module games.GameModule
---@return games.DescribeCtx
local function describe_ctx(game, module)
  local options = require("games").open_options()
  return {
    art = require("games.ui.assets").module_art(game, module, options),
    ambiwidth = vim.o.ambiwidth --[[@as games.Ambiwidth]],
  }
end

---@param update games.GameUpdate
---@param ctx games.DescribeCtx
---@return games.Talk
function M.talk(update, ctx)
  local art, ambiwidth = ctx.art, ctx.ambiwidth
  return {
    name = function(seat)
      return players.name_of_seat(update, seat)
    end,
    art = art,
    ambiwidth = ambiwidth,
    cards = function(shown)
      return cards.text(assert(art, "cards draw from an asset pack"), shown, ambiwidth)
    end,
    faces = function(faces)
      return dice.text(assert(art, "dice draw from an asset pack"), faces, ambiwidth)
    end,
  }
end

local function says_nothing()
  return nil
end

---@param update games.GameUpdate
---@return fun(event: games.Event): string|nil what the game's module says of an event, if anything
local function module_describer(update)
  local loaded, module = pcall(registry.game_module, update.game)
  if not (loaded and module and (module.describers or module.describe_event)) then
    return says_nothing
  end
  local ctx = describe_ctx(update.game, module)
  local describers = module.describers
  if describers then
    local talk = M.talk(update, ctx)
    return function(event)
      local describer = describers[event.kind]
      return describer and describer(event, talk)
    end
  end
  local describe_event = module.describe_event
  ---@cast describe_event -nil
  return function(event)
    return describe_event(event, update, ctx)
  end
end

---A game module that fails to load falls back to generic descriptions, so listings still work.
---@param update games.GameUpdate
---@return string
function M.events(update)
  local describe_event = module_describer(update)
  local descriptions = vim.tbl_map(function(event)
    return describe_event(event) or fallback_description(event, update)
  end, update.events)
  return table.concat(descriptions, "; ")
end

return M
