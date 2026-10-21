local board = require("games.ui.widgets.board")
local cards = require("games.ui.cards")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local tables = require("games.core.tables")

---@alias games.UnoColor "R"|"Y"|"G"|"B"|"" none only before the colour of a Wild turned up is named
---@alias games.UnoPhase "color"|"play"|"drawn"|"challenge"
---@alias games.UnoDirection "clockwise"|"counterclockwise"

---@class games.UnoSeatView
---@field seat games.Seat
---@field cards integer
---@field out boolean left the game

---@class games.UnoView
---@field top games.UnoCardKey
---@field color games.UnoColor the colour in play, which a wild on top names
---@field direction games.UnoDirection
---@field phase games.UnoPhase
---@field draw_pile integer
---@field seats games.UnoSeatView[]
---@field my_hand games.UnoCardKey[]
---@field legal games.ActionKind[] the viewer's own: empty unless it is to act
---@field playable games.UnoCardKey[] the viewer's own: the cards it may play now

---@class games.UnoUpdate: games.GameUpdate
---@field view games.UnoView

---@class games.UnoStartedEvent: games.Event
---@field kind "started"
---@field by games.Seat
---@field dealer games.Seat
---@field top games.UnoCardKey

---@class games.UnoPlayedEvent: games.Event
---@field kind "played"
---@field by games.Seat
---@field card games.UnoCardKey
---@field color games.UnoColor

---@class games.UnoDrewEvent: games.Event
---@field kind "drew"
---@field by games.Seat
---@field seat games.Seat
---@field count integer

---@class games.UnoPassedEvent: games.Event
---@field kind "passed"
---@field by games.Seat

---@class games.UnoSkippedEvent: games.Event
---@field kind "skipped"
---@field by games.Seat
---@field seat games.Seat

---@class games.UnoReversedEvent: games.Event
---@field kind "reversed"
---@field by games.Seat
---@field direction games.UnoDirection

---@class games.UnoColorEvent: games.Event
---@field kind "color"
---@field by games.Seat
---@field color games.UnoColor

---@class games.UnoChallengedEvent: games.Event
---@field kind "challenged"
---@field by games.Seat
---@field seat games.Seat
---@field guilty boolean

---@class games.UnoReshuffledEvent: games.Event
---@field kind "reshuffled"
---@field by games.Seat
---@field cards integer

---@class games.UnoLeftEvent: games.Event
---@field kind "left"
---@field by games.Seat

local DECK_KEY = "deck"
local CARD_HL = "GamesSquareLight"
local KEY_HL = "GamesKey"
local MUTED_HL = "GamesMuted"
local PLAY_KIND = "play"
---Display columns the table panel reserves: nine cards to a line of the hand.
local PANEL_WIDTH = 36

---@type table<string, string> colour letter -> its name
local COLOR_NAMES = {}
for _, color in ipairs(cards.uno_colors) do
  COLOR_NAMES[color.letter] = color.name
end

---@type table<games.UnoPhase, string> what the seat to act does in each phase
local PHASE_HINTS = {
  color = "name the colour",
  play = "play a card or draw",
  drawn = "play the card drawn or keep it",
  challenge = "challenge the Wild Draw Four or accept it",
}
---@type table<games.UnoPhase, string> what the seat to act does in a phase when it can play nothing
local NOTHING_TO_PLAY_HINTS =
  { play = "nothing to play: draw", drawn = "it does not match: keep it" }

---@param n integer
---@return string
local function card_count(n)
  return n == 1 and "1 card" or ("%d cards"):format(n)
end

---@param color games.UnoColor
---@return string
local function color_text(color)
  local name = COLOR_NAMES[color]
  return name and ("%s (%s)"):format(name, color) or "not named yet"
end

---@param update games.GameUpdate
---@param kind games.ActionKind
---@return boolean
local function legal(update, kind)
  return vim.tbl_contains(update.view.legal, kind)
end

---@param view games.UnoView
---@return string|nil
local function hint(view)
  if #view.legal == 0 then
    return nil
  end
  if #view.playable == 0 and NOTHING_TO_PLAY_HINTS[view.phase] then
    return NOTHING_TO_PLAY_HINTS[view.phase]
  end
  return PHASE_HINTS[view.phase]
end

---@param view games.UnoView
---@return games.KvPair[]
local function table_pairs(view)
  local pairs = {
    { "Colour", color_text(view.color) },
    { "Turns", view.direction },
    { "Draw pile", card_count(view.draw_pile) },
  }
  local now = hint(view)
  if now then
    pairs[#pairs + 1] = { "Now", now }
  end
  return pairs
end

---@param view games.UnoView
---@param ctx games.ArtLayoutCtx
---@return games.Node[]
local function hand_lines(view, ctx)
  if #view.my_hand == 0 then
    return {}
  end
  local lines = { compose.text(""), compose.text("Your hand", KEY_HL) }
  return vim.list_extend(
    lines,
    cards.hand({
      set = ctx.art,
      cards = view.my_hand,
      marked = tables.set_of(view.playable),
      glyph_width = cards.UNO_GLYPH_WIDTH,
      width = PANEL_WIDTH,
      ambiwidth = ctx.ambiwidth,
      hl = cards.uno_hl,
    })
  )
end

local M = {}

---@type string[] keys every uno asset pack draws: the 54 distinct cards and the draw pile
M.asset_keys = vim.list_extend(cards.uno_keys(), { DECK_KEY })

M.glyph_width = cards.UNO_GLYPH_WIDTH

---The discard pile's top card in the colour in play beside the draw pile, with the table and
---your hand, the cards you may play marked, in the panel.
---@param update games.UnoUpdate
---@param ctx games.ArtLayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = 1,
      cols = 2,
      cell_width = cards.UNO_GLYPH_WIDTH,
      cell = function(_, c)
        if c == 1 then
          return { view.top, cards.uno_hl(view.color) }
        end
        return { DECK_KEY, MUTED_HL }
      end,
      marks = { grid.mark(1, 1, CARD_HL), grid.mark(1, 2, CARD_HL) },
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          local nodes = { kv({ pairs = table_pairs(view), key_hl = KEY_HL }) }
          return compose.col(vim.list_extend(nodes, hand_lines(view, ctx)))
        end,
      },
    },
  }, ctx)
end

---Each seat's cards left, "UNO" on its last, after the clock on its header line.
---@param update games.UnoUpdate
---@param seat games.Seat
---@return games.TextNode[]|nil
function M.player_extra(update, seat)
  local entry = players.entry_of_seat(update.view.seats, seat)
  if not entry then
    return nil
  end
  if entry.out then
    return { compose.text("out", MUTED_HL) }
  end
  local nodes = { compose.text(card_count(entry.cards), KEY_HL) }
  if entry.cards == 1 then
    nodes[#nodes + 1] = compose.text(" UNO", KEY_HL)
  end
  return nodes
end

---@type games.GameInput
M.input = {
  kind = PLAY_KIND,
  prompt = "Play, e.g. r5 or w g: ",
}

---@param key string
---@param label string
---@param kind games.ActionKind
---@return games.GameAction
local function simple(key, label, kind)
  return {
    key = key,
    label = label,
    action = { kind = kind },
    available = function(update)
      return legal(update, kind)
    end,
  }
end

---@type games.GameAction[]
M.actions = {
  simple("d", "draw", "draw"),
  simple("k", "keep", "pass"),
  simple("c", "challenge", "challenge"),
  simple("a", "accept", "accept"),
}
for _, color in ipairs(cards.uno_colors) do
  M.actions[#M.actions + 1] = {
    key = color.letter:lower(),
    label = color.name,
    action = { kind = "color", color = color.letter },
    available = function(update)
      return legal(update, "color")
    end,
  }
end

---@param talk games.Talk
---@param card games.UnoCardKey
---@return string its glyph, without the padding that lines a hand up
local function card_text(talk, card)
  return vim.trim(talk.cards({ card }))
end

---@type table<games.EventKind, games.Describer>
M.describers = {
  started = function(event, talk)
    return ("%s deals and turns up %s"):format(talk.name(event.dealer), card_text(talk, event.top))
  end,
  played = function(event, talk)
    local text = ("%s plays %s"):format(talk.name(event.by), card_text(talk, event.card))
    if not vim.tbl_contains(cards.UNO_WILDS, event.card) then
      return text
    end
    return ("%s and names %s"):format(text, COLOR_NAMES[event.color])
  end,
  drew = function(event, talk)
    return ("%s draws %s"):format(talk.name(event.seat), card_count(event.count))
  end,
  passed = function(event, talk)
    return ("%s keeps the card drawn"):format(talk.name(event.by))
  end,
  skipped = function(event, talk)
    return ("%s is skipped"):format(talk.name(event.seat))
  end,
  reversed = function(event)
    return ("play turns %s"):format(event.direction)
  end,
  color = function(event, talk)
    return ("%s names %s"):format(talk.name(event.by), COLOR_NAMES[event.color])
  end,
  challenged = function(event, talk)
    return ("%s challenges %s's Wild Draw Four: %s"):format(
      talk.name(event.by),
      talk.name(event.seat),
      event.guilty and "guilty" or "not guilty"
    )
  end,
  reshuffled = function(event)
    return ("the discards are shuffled into a new draw pile of %s"):format(card_count(event.cards))
  end,
  left = function(event, talk)
    return ("%s left the game"):format(talk.name(event.by))
  end,
}

return M
