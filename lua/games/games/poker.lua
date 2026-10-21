local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local cards = require("games.ui.cards")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local text_util = require("games.ui.text")

local Group = highlights.Group

---@alias games.PokerCard games.CardKey
---@alias games.PokerMove "fold"|"check"|"call"|"bet"|"raise"|"allin"

---@class games.PokerSeatView
---@field seat games.Seat
---@field user games.UserID
---@field stack integer
---@field bet integer this street's bet
---@field folded boolean
---@field allin boolean
---@field out boolean

---@class games.PokerPot
---@field amount integer
---@field eligible games.Seat[]

---@class games.PokerShownHand
---@field seat games.Seat
---@field cards games.PokerCard[]
---@field category string e.g. "two_pair"

---@class games.PokerAward
---@field seat games.Seat
---@field amount integer
---@field pot integer 0 for the main pot

---@class games.PokerHandSummary
---@field hand_no integer
---@field board games.PokerCard[]
---@field shown games.PokerShownHand[]
---@field awards games.PokerAward[]

---@class games.PokerBlinds
---@field small integer
---@field big integer

---@class games.PokerView
---@field hand_no integer
---@field button games.Seat
---@field blinds games.PokerBlinds
---@field board games.PokerCard[]
---@field pots games.PokerPot[] settled on earlier streets
---@field seats games.PokerSeatView[]
---@field legal games.PokerMove[] the viewer's own: empty unless it is to act
---@field to_call integer
---@field min_raise_to integer 0 unless legal holds bet or raise
---@field max_raise_to integer
---@field my_cards games.PokerCard[]
---@field last_hand? games.PokerHandSummary

---@class games.PokerUpdate: games.GameUpdate
---@field view games.PokerView

---@class games.PokerHandStartedEvent: games.Event
---@field kind "hand_started"
---@field by games.Seat
---@field hand_no integer
---@field button games.Seat
---@field blinds games.PokerBlinds

---@class games.PokerPostedEvent: games.Event
---@field kind "posted"
---@field by games.Seat
---@field seat games.Seat
---@field blind string "small" or "big"
---@field amount integer less than the blind when it put the seat all-in

---@class games.PokerActedEvent: games.Event
---@field kind "acted"
---@field by games.Seat
---@field move games.PokerMove
---@field amount integer the seat's total for the street after it, 0 for fold and check

---@class games.PokerBoardEvent: games.Event
---@field kind "board"
---@field by games.Seat
---@field street string
---@field cards games.PokerCard[]

---@class games.PokerShownEvent: games.Event
---@field kind "shown"
---@field by games.Seat
---@field hands games.PokerShownHand[]

---@class games.PokerReturnedEvent: games.Event
---@field kind "returned"
---@field by games.Seat
---@field seat games.Seat
---@field amount integer the part of its bet no one called

---@class games.PokerWonEvent: games.Event
---@field kind "won"
---@field by games.Seat
---@field seat games.Seat
---@field amount integer
---@field pot integer 0 for the main pot

---@class games.PokerBustedEvent: games.Event
---@field kind "busted"
---@field by games.Seat
---@field seat games.Seat

---@class games.PokerLeftEvent: games.Event
---@field kind "left"
---@field by games.Seat

local BOARD_SIZE = 5
local BOARD_ROW, HAND_ROW = 1, 2
local EMPTY_KEY = "empty"
local BLANK_KEY = "blank"
local BUTTON_KEY = "button"
local CARD_HL = Group.SQUARE_LIGHT
---Display columns the table panel reserves: "Moves    fold call raise allin".
local PANEL_WIDTH = 32

---What each move reads as in an event; an amount is the seat's total for the street.
---@type table<games.PokerMove, string>
local MOVE_VERBS = {
  fold = "folds",
  check = "checks",
  call = "calls to %d",
  bet = "bets %d",
  raise = "raises to %d",
  allin = "goes all-in to %d",
}

---@param view games.PokerView
---@param row integer
---@param col integer
---@return games.PokerCard|nil
local function card_at(view, row, col)
  if row == BOARD_ROW then
    return view.board[col]
  end
  return view.my_cards[col]
end

---@param view games.PokerView
---@param row integer
---@param col integer
---@return games.GridCell
local function cell_for(view, row, col)
  local card = card_at(view, row, col)
  if card then
    return { card, cards.hl(card) }
  end
  return { row == BOARD_ROW and EMPTY_KEY or BLANK_KEY }
end

---@param view games.PokerView
---@return games.GridMark[]
local function card_marks(view)
  local marks = {}
  for row = BOARD_ROW, HAND_ROW do
    for col = 1, BOARD_SIZE do
      if card_at(view, row, col) then
        marks[#marks + 1] = grid.mark(row, col, CARD_HL)
      end
    end
  end
  return marks
end

---@param update games.PokerUpdate
---@param seat games.Seat
---@return string
local function name_of(update, seat)
  return vim.fn.strcharpart(players.name_of_seat(update, seat), 0, text_util.NAME_WIDTH)
end

---@param pots games.PokerPot[]
---@return string
local function pots_text(pots)
  if #pots == 0 then
    return "0"
  end
  local amounts = vim.tbl_map(function(pot)
    return tostring(pot.amount)
  end, pots)
  return table.concat(amounts, " + side ")
end

---@param view games.PokerView
---@return games.KvPair[]
local function betting_pairs(view)
  if #view.legal == 0 then
    return {}
  end
  local pairs = { { "To call", tostring(view.to_call) } }
  if view.max_raise_to > 0 then
    local label = vim.tbl_contains(view.legal, "bet") and "Bet" or "Raise to"
    pairs[#pairs + 1] = { label, ("%d-%d"):format(view.min_raise_to, view.max_raise_to) }
  end
  pairs[#pairs + 1] = { "Moves", table.concat(view.legal, " ") }
  return pairs
end

---@param view games.PokerView
---@return games.Node
local function table_node(view)
  local pairs = {
    { "Hand", ("%d  blinds %d/%d"):format(view.hand_no, view.blinds.small, view.blinds.big) },
    { "Pot", pots_text(view.pots) },
  }
  return kv({ pairs = vim.list_extend(pairs, betting_pairs(view)), key_hl = Group.KEY })
end

---@param update games.PokerUpdate
---@param award games.PokerAward
---@return string
local function award_text(update, award)
  local pot = award.pot > 0 and (" (side %d)"):format(award.pot) or ""
  return ("%s won %d%s"):format(name_of(update, award.seat), award.amount, pot)
end

---@param update games.PokerUpdate
---@param set games.ArtSet
---@param ambiwidth games.Ambiwidth
---@return games.Node[]
local function last_hand_lines(update, set, ambiwidth)
  local last = update.view.last_hand
  if not last then
    return {}
  end
  ---@type games.Node[]
  local lines = { compose.text(""), compose.text(("Last hand %d"):format(last.hand_no), Group.KEY) }
  for _, shown in ipairs(last.shown) do
    local name =
      text_util.fit_to_width(players.name_of_seat(update, shown.seat), text_util.NAME_WIDTH)
    local parts = { compose.text(name .. " ") }
    vim.list_extend(parts, cards.nodes(set, shown.cards, ambiwidth))
    parts[#parts + 1] = compose.text("  " .. text_util.humanize(shown.category))
    lines[#lines + 1] = compose.row(parts)
  end
  for _, award in ipairs(last.awards) do
    lines[#lines + 1] = compose.text(award_text(update, award), Group.MUTED)
  end
  return lines
end

---@param seat games.PokerSeatView
---@return string
local function bet_text(seat)
  return seat.bet > 0 and ("bet %d"):format(seat.bet) or ""
end

---@param seat games.PokerSeatView
---@return string
local function status_text(seat)
  if seat.out then
    return "out"
  end
  if seat.folded then
    return "folded"
  end
  return seat.allin and "all-in" or ""
end

---@param view games.PokerView
---@param seat games.Seat
---@param ctx games.ArtLayoutCtx
---@return string the button's glyph on its seat, blank on the others
local function button_mark(view, seat, ctx)
  local set = ctx.art
  return art.glyph(set, view.button == seat and BUTTON_KEY or BLANK_KEY, ctx.ambiwidth)
end

local M = {}

---@type string[] keys every poker asset pack draws: the 52 cards, an empty board slot, a blank
---and the dealer button
M.asset_keys = vim.list_extend(cards.keys(), { EMPTY_KEY, BLANK_KEY, BUTTON_KEY })

M.glyph_width = cards.GLYPH_WIDTH

---The seat's row of the seats table: the button, its stack, its bet this street and whether it
---folded, is all-in or out, each column as wide as its widest entry at the table.
---@param update games.PokerUpdate
---@param seat games.Seat
---@param ctx games.ArtLayoutCtx
---@return games.TextNode[]|nil
function M.player_extra(update, seat, ctx)
  local view = update.view
  local entry = players.entry_of_seat(view.seats, seat)
  if not entry then
    return nil
  end
  local stack_width = text_util.widest(view.seats, function(s)
    return tostring(s.stack)
  end)
  local bet_width = text_util.widest(view.seats, bet_text)
  return {
    compose.text(button_mark(view, seat, ctx), Group.KEY),
    compose.text(" " .. text_util.align_right(tostring(entry.stack), stack_width)),
    compose.text(" " .. text_util.pad_to_width(bet_text(entry), bet_width)),
    compose.text(" " .. status_text(entry), Group.MUTED),
  }
end

---@param update games.PokerUpdate
---@param ctx games.ArtLayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local set = ctx.art
  return board({
    grid = {
      rows = HAND_ROW,
      cols = BOARD_SIZE,
      cell_width = cards.GLYPH_WIDTH,
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = card_marks(view),
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return compose.col(
            vim.list_extend({ table_node(view) }, last_hand_lines(update, set, ctx.ambiwidth))
          )
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = "act",
  prompt = "Bet/raise/allin: ",
}

---@param update games.GameUpdate
---@param ... games.PokerMove
---@return boolean
local function legal_any(update, ...)
  local legal = update.view.legal
  return vim.iter({ ... }):any(function(move)
    return vim.tbl_contains(legal, move)
  end)
end

---@param view games.PokerView
---@return games.PokerMove the one of check and call the seat to act may make
local function check_or_call(view)
  return vim.tbl_contains(view.legal, "check") and "check" or "call"
end

---@type games.GameAction[]
M.actions = {
  {
    key = "f",
    label = "fold",
    action = { kind = "act", text = "fold" },
    available = function(update)
      return legal_any(update, "fold")
    end,
  },
  {
    key = "c",
    label = function(update)
      local move = check_or_call(update.view)
      return move == "call" and ("call %d"):format(update.view.to_call) or move
    end,
    act = function(update)
      return { kind = "act", text = check_or_call(update.view) }
    end,
    available = function(update)
      return legal_any(update, "check", "call")
    end,
  },
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  hand_started = function(event)
    return ("hand %d, blinds %d/%d"):format(event.hand_no, event.blinds.small, event.blinds.big)
  end,
  posted = function(event, talk)
    return ("%s posts %d"):format(talk.name(event.seat), event.amount)
  end,
  acted = function(event, talk)
    return ("%s %s"):format(talk.name(event.by), MOVE_VERBS[event.move]:format(event.amount))
  end,
  board = function(event, talk)
    return ("%s %s"):format(event.street, talk.cards(event.cards))
  end,
  shown = function(event, talk)
    local hands = vim.tbl_map(function(hand)
      return ("%s %s"):format(talk.name(hand.seat), talk.cards(hand.cards))
    end, event.hands)
    return "showdown: " .. table.concat(hands, ", ")
  end,
  returned = function(event, talk)
    return ("%d returned to %s"):format(event.amount, talk.name(event.seat))
  end,
  won = function(event, talk)
    local pot = event.pot > 0 and (" (side %d)"):format(event.pot) or ""
    return ("%s wins %d%s"):format(talk.name(event.seat), event.amount, pot)
  end,
  busted = function(event, talk)
    return ("%s is out"):format(talk.name(event.seat))
  end,
  left = function(event, talk)
    return ("%s left the table"):format(talk.name(event.by))
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function card(key)
    return grid.sample(ctx, { key, cards.hl(key) }, CARD_HL)
  end
  return {
    {
      sample = { card("Qs"), compose.text(" "), card("Ah") },
      meaning = "a card: the board's on top, yours below",
    },
    { sample = { grid.sample(ctx, { EMPTY_KEY }) }, meaning = "a board card still to come" },
    {
      sample = { grid.sample(ctx, { BUTTON_KEY, Group.KEY }) },
      meaning = "the dealer button, beside its seat",
    },
  }
end

return M
