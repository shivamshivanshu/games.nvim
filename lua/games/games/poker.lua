local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local cards = require("games.ui.cards")
local compose = require("games.ui.compose")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local text_util = require("games.ui.text")

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

---@class games.PokerView
---@field hand_no integer
---@field button games.Seat
---@field blinds {small: integer, big: integer}
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

local BOARD_SIZE = 5
local BOARD_ROW, HAND_ROW = 1, 2
local GLYPH_WIDTH = 2
local EMPTY_KEY = "empty"
local BLANK_KEY = "blank"
local BUTTON_KEY = "button"
local CARD_HL = "GamesSquareLight"
local KEY_HL = "GamesKey"
local MUTED_HL = "GamesMuted"
---Display columns the table panel reserves: "Moves    fold call raise allin".
local PANEL_WIDTH = 32
---Display columns a user name may take in the last-hand lines.
local NAME_WIDTH = 10

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
        marks[#marks + 1] = { row = row, col = col, hl = CARD_HL }
      end
    end
  end
  return marks
end

---@param update games.PokerUpdate
---@param seat games.Seat
---@return string
local function name_of(update, seat)
  local user = players.user_of_seat(update, seat) or seat
  return vim.fn.strcharpart(user, 0, NAME_WIDTH)
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
  return kv({ pairs = vim.list_extend(pairs, betting_pairs(view)), key_hl = KEY_HL })
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
  local lines = { compose.text(""), compose.text(("Last hand %d"):format(last.hand_no), KEY_HL) }
  for _, shown in ipairs(last.shown) do
    local name = text_util.pad_to_width(name_of(update, shown.seat), NAME_WIDTH)
    local parts = { compose.text(name .. " ") }
    vim.list_extend(parts, cards.nodes(set, shown.cards, ambiwidth))
    parts[#parts + 1] = compose.text("  " .. text_util.humanize(shown.category))
    lines[#lines + 1] = compose.row(parts)
  end
  for _, award in ipairs(last.awards) do
    lines[#lines + 1] = compose.text(award_text(update, award), MUTED_HL)
  end
  return lines
end

---@param seats games.PokerSeatView[]
---@param field fun(seat: games.PokerSeatView): string
---@return integer
local function widest(seats, field)
  return text_util.max_display_width(vim.tbl_map(field, seats))
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
---@return games.PokerSeatView|nil
local function seat_view(view, seat)
  return vim.iter(view.seats):find(function(entry)
    return entry.seat == seat
  end)
end

---@param view games.PokerView
---@param seat games.Seat
---@param ctx games.LayoutCtx
---@return string the button's glyph on its seat, blank on the others
local function button_mark(view, seat, ctx)
  local set = assert(ctx.art, "poker draws from an asset pack")
  return art.glyph(set, view.button == seat and BUTTON_KEY or BLANK_KEY, ctx.ambiwidth)
end

local M = {}

---@type string[] keys every poker asset pack draws: the 52 cards, an empty board slot, a blank
---and the dealer button
M.asset_keys = vim.list_extend(cards.keys(), { EMPTY_KEY, BLANK_KEY, BUTTON_KEY })

M.glyph_width = GLYPH_WIDTH

---The seat's row of the seats table: the button, its stack, its bet this street and whether it
---folded, is all-in or out, each column as wide as its widest entry at the table.
---@param update games.PokerUpdate
---@param seat games.Seat
---@param ctx games.LayoutCtx
---@return games.TextNode[]|nil
function M.player_extra(update, seat, ctx)
  local view = update.view
  local entry = seat_view(view, seat)
  if not entry then
    return nil
  end
  local stack_width = widest(view.seats, function(s)
    return tostring(s.stack)
  end)
  local bet_width = widest(view.seats, bet_text)
  return {
    compose.text(button_mark(view, seat, ctx), KEY_HL),
    compose.text(" " .. text_util.align_right(tostring(entry.stack), stack_width)),
    compose.text(" " .. text_util.pad_to_width(bet_text(entry), bet_width)),
    compose.text(" " .. status_text(entry), MUTED_HL),
  }
end

---@param update games.PokerUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local set = assert(ctx.art, "poker draws from an asset pack")
  return board({
    grid = {
      rows = HAND_ROW,
      cols = BOARD_SIZE,
      shade = "plain",
      cell_width = GLYPH_WIDTH,
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = card_marks(view),
    },
    art = set,
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
      local view = update.view
      return vim.tbl_contains(view.legal, "check") and "check" or ("call %d"):format(view.to_call)
    end,
    action = { kind = "act", text = "call" },
    available = function(update)
      return legal_any(update, "check", "call")
    end,
  },
}

---@class (private) games.PokerTalk how events name seats and cards
---@field name fun(seat: games.Seat): string
---@field cards fun(cards: games.PokerCard[]): string

---@type table<string, fun(event: table, talk: games.PokerTalk): string>
local DESCRIBERS = {
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

---@param event games.Event
---@param update games.PokerUpdate
---@param ctx games.DescribeCtx with art, which a module with asset_keys always gets
---@return string|nil
function M.describe_event(event, update, ctx)
  local describer = DESCRIBERS[event.kind]
  if not describer then
    return nil
  end
  ---@type games.PokerTalk
  local talk = {
    name = function(seat)
      return players.user_of_seat(update, seat) or seat
    end,
    cards = function(shown)
      return cards.text(ctx.art --[[@as games.ArtSet]], shown, ctx.ambiwidth)
    end,
  }
  return describer(event, talk)
end

return M
