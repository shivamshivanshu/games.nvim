local board = require("games.ui.widgets.board")
local cards = require("games.ui.cards")
local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local tables = require("games.core.tables")
local text_util = require("games.ui.text")

local Group = highlights.Group

---@alias games.HeartsPass "left"|"right"|"across"|"hold"
---@alias games.HeartsPhase "pass"|"play"
---@alias games.HeartsPrompt ""|"pass"|"wait"|"lead"|"play" what the viewer is to do now; "" for nothing

---@class games.HeartsSeatView
---@field seat games.Seat
---@field score integer
---@field taken integer points taken in this hand's tricks so far
---@field pass_chosen boolean
---@field out boolean left the game; its cards are played for it

---@class games.HeartsPlay
---@field seat games.Seat
---@field card games.CardKey

---@class games.HeartsTrickResult
---@field plays games.HeartsPlay[]
---@field winner games.Seat
---@field points integer

---@class games.HeartsSeatPoints
---@field seat games.Seat
---@field points integer

---@class games.HeartsHandScore
---@field hand_no integer
---@field points games.HeartsSeatPoints[]
---@field moon? games.Seat the seat that took every point, if one did

---@class games.HeartsView
---@field hand_no integer
---@field pass games.HeartsPass
---@field phase games.HeartsPhase
---@field score_limit integer
---@field seats games.HeartsSeatView[]
---@field trick games.HeartsPlay[] the trick in play, in play order
---@field last_trick? games.HeartsTrickResult
---@field hearts_broken boolean
---@field my_hand games.CardKey[]
---@field legal games.CardKey[] the viewer's own: the cards it may play now
---@field must_pass integer the viewer's own: cards it has still to choose to pass, else 0
---@field pass_to? games.Seat the viewer's own: whom its passes go to; nil in a hand without passing
---@field received_from? games.Seat the viewer's own: who passes to it; nil in a hand without passing
---@field passed games.CardKey[] the viewer's own: the cards it chose to pass this hand
---@field received games.CardKey[] the viewer's own: the cards passed to it this hand
---@field last_hand? games.HeartsHandScore
---@field prompt games.HeartsPrompt the viewer's own

---@class games.HeartsUpdate: games.GameUpdate
---@field view games.HeartsView

---@class games.HeartsHandStartedEvent: games.Event
---@field kind "hand_started"
---@field by games.Seat
---@field hand_no integer
---@field pass games.HeartsPass

---@class games.HeartsPassChosenEvent: games.Event
---@field kind "pass_chosen"
---@field by games.Seat
---@field seat games.Seat

---@class games.HeartsPassedEvent: games.Event
---@field kind "passed"
---@field by games.Seat
---@field pass games.HeartsPass

---@class games.HeartsPlayedEvent: games.Event
---@field kind "played"
---@field by games.Seat
---@field seat games.Seat
---@field card games.CardKey

---@class games.HeartsTrickWonEvent: games.Event
---@field kind "trick_won"
---@field by games.Seat
---@field seat games.Seat
---@field points integer

---@class games.HeartsLeftEvent: games.Event
---@field kind "left"
---@field by games.Seat

---@class games.HeartsHandScoredEvent: games.Event
---@field kind "hand_scored"
---@field by games.Seat
---@field score games.HeartsHandScore

local EMPTY_KEY = "empty"
local BLANK_KEY = "blank"
local CARD_HL = Group.SQUARE_LIGHT
local PLAY_KIND = "play"
local PASS_KIND = "pass"
local HOLD = "hold"
local SEATS = 4
---Display columns the table panel reserves: a full hand of 13 cards on one line.
local PANEL_WIDTH = 38

---Where each seat sits round the trick, by its offset clockwise from the viewer, who sits at
---the bottom: the next seat is on its left.
---@type {row: integer, col: integer, name: string}[]
local PLACES = {
  [0] = { row = 3, col = 2, name = "" },
  { row = 2, col = 1, name = "left" },
  { row = 1, col = 2, name = "across" },
  { row = 2, col = 3, name = "right" },
}
---@type games.HeartsPrompt
local PASS_PROMPT = "pass"
---@type table<games.HeartsPrompt, string> the text of each prompt; the pass's takes the cards to
---choose and whom they go to
local PROMPT_TEXT = {
  pass = "pass %d cards to %s",
  wait = "wait for the others to pass",
  lead = "lead a card",
  play = "play a card",
}
local PLACE_WIDTH = text_util.widest(vim.tbl_values(PLACES), function(place)
  return place.name
end)

---@param n integer
---@return string
local function points_text(n)
  return n == 1 and "1 point" or ("%d points"):format(n)
end

---@param pass games.HeartsPass
---@return string
local function passing_text(pass)
  return pass == HOLD and "no passing" or ("passing " .. pass)
end

---The position in view.seats of the seat at the bottom: the viewer's, or the first seat for a
---spectator.
---@param update games.HeartsUpdate
---@param me games.UserID
---@return integer
local function bottom_index(update, me)
  local mine = players.seat_of_user(update, me)
  for i, entry in ipairs(update.view.seats) do
    if entry.seat == mine then
      return i
    end
  end
  return 1
end

---@param update games.HeartsUpdate
---@param me games.UserID
---@param offset integer clockwise from the bottom seat
---@return games.Seat
local function seat_at(update, me, offset)
  local index = (bottom_index(update, me) - 1 + offset) % SEATS + 1
  return update.view.seats[index].seat
end

---@param update games.HeartsUpdate
---@param me games.UserID
---@param seat games.Seat
---@return integer offset clockwise from the bottom seat
local function offset_of(update, me, seat)
  for offset = 0, SEATS - 1 do
    if seat_at(update, me, offset) == seat then
      return offset
    end
  end
  error(("hearts: no seat %q"):format(seat))
end

---@param update games.HeartsUpdate
---@param me games.UserID
---@return table<integer, table<integer, games.GridCell>> by row and column, the seats' places
---@return games.GridMark[] the card background under each card played
local function trick_cells(update, me)
  local played = {}
  for _, play in ipairs(update.view.trick) do
    played[play.seat] = play.card
  end
  ---@type table<integer, table<integer, games.GridCell>>
  local cells = { {}, {}, {} }
  local marks = {}
  for offset = 0, SEATS - 1 do
    local place = PLACES[offset]
    local card = played[seat_at(update, me, offset)]
    cells[place.row][place.col] = card and { card, cards.hl(card) } or { EMPTY_KEY, Group.MUTED }
    if card then
      marks[#marks + 1] = grid.mark(place.row, place.col, CARD_HL)
    end
  end
  return cells, marks
end

---@param view games.HeartsView
---@param talk games.Talk
---@return string|nil what the viewer is to do now
local function prompt_text(view, talk)
  local text = PROMPT_TEXT[view.prompt]
  if view.prompt == PASS_PROMPT then
    return text:format(view.must_pass, talk.name(view.pass_to))
  end
  return text
end

---@param update games.HeartsUpdate
---@param talk games.Talk
---@return games.KvPair[]
local function table_pairs(update, talk)
  local view = update.view
  local pairs = {
    { "Hand", ("%d, %s"):format(view.hand_no, passing_text(view.pass)) },
    { "Hearts", view.hearts_broken and "broken" or "not broken" },
  }
  local last = view.last_trick
  if last then
    local shown = vim.tbl_map(function(play)
      return play.card
    end, last.plays)
    pairs[#pairs + 1] = {
      "Last trick",
      ("%s  %s took %s"):format(
        talk.cards(shown),
        talk.name(last.winner),
        points_text(last.points)
      ),
    }
  end
  if #view.passed > 0 and view.pass_to then
    pairs[#pairs + 1] =
      { "Passed", ("%s to %s"):format(talk.cards(view.passed), talk.name(view.pass_to)) }
  end
  if #view.received > 0 and view.received_from then
    pairs[#pairs + 1] = {
      "Received",
      ("%s from %s"):format(talk.cards(view.received), talk.name(view.received_from)),
    }
  end
  local now = prompt_text(view, talk)
  if now then
    pairs[#pairs + 1] = { "Now", now }
  end
  return pairs
end

---@param view games.HeartsView
---@param ctx games.ArtLayoutCtx
---@return games.Node[]
local function hand_lines(view, ctx)
  if #view.my_hand == 0 then
    return {}
  end
  local lines = { compose.text(""), compose.text("Your hand", Group.KEY) }
  return vim.list_extend(
    lines,
    cards.hand({
      set = ctx.art,
      cards = view.my_hand,
      marked = tables.set_of(view.legal),
      glyph_width = cards.GLYPH_WIDTH,
      width = PANEL_WIDTH,
      ambiwidth = ctx.ambiwidth,
    })
  )
end

---@param last games.HeartsHandScore|nil
---@param talk games.Talk
---@return games.Node[]
local function last_hand_lines(last, talk)
  if not last then
    return {}
  end
  local lines = { compose.text(""), compose.text(("Last hand %d"):format(last.hand_no), Group.KEY) }
  for _, entry in ipairs(last.points) do
    local name = text_util.fit_to_width(talk.name(entry.seat), text_util.NAME_WIDTH)
    lines[#lines + 1] = compose.text(("%s +%d"):format(name, entry.points))
  end
  if last.moon then
    lines[#lines + 1] = compose.text(("%s shot the moon"):format(talk.name(last.moon)), Group.MUTED)
  end
  return lines
end

local M = {}

---@type string[] keys every hearts asset pack draws: the 52 cards, an empty place in the trick
---and a blank
M.asset_keys = vim.list_extend(cards.keys(), { EMPTY_KEY, BLANK_KEY })

M.glyph_width = cards.GLYPH_WIDTH

---The trick round the table, you at the bottom and the seat on your left on the left, with the
---table, your hand, the cards you may play marked, and the last hand's scores in the panel.
---@param update games.HeartsUpdate
---@param ctx games.ArtLayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local talk = describe.talk(update, { art = ctx.art, ambiwidth = ctx.ambiwidth })
  local cells, marks = trick_cells(update, ctx.me)
  return board({
    grid = {
      rows = 3,
      cols = 3,
      cell_width = cards.GLYPH_WIDTH,
      cell = function(r, c)
        return cells[r][c] or { BLANK_KEY }
      end,
      marks = marks,
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          local nodes = { kv({ pairs = table_pairs(update, talk), key_hl = Group.KEY }) }
          vim.list_extend(nodes, hand_lines(view, ctx))
          return compose.col(vim.list_extend(nodes, last_hand_lines(view.last_hand, talk)))
        end,
      },
    },
  }, ctx)
end

---@param seat games.HeartsSeatView
---@return string
local function taken_text(seat)
  return seat.taken > 0 and ("+" .. seat.taken) or ""
end

---The seat's score, the points it has taken this hand, where it sits from you, and whether it
---has chosen its passes or left, each column as wide as its widest entry at the table.
---@param update games.HeartsUpdate
---@param seat games.Seat
---@param ctx games.ArtLayoutCtx
---@return games.TextNode[]|nil
function M.player_extra(update, seat, ctx)
  local view = update.view
  local entry = players.entry_of_seat(view.seats, seat)
  if not entry then
    return nil
  end
  local score_width = text_util.widest(view.seats, function(s)
    return tostring(s.score)
  end)
  local taken_width = text_util.widest(view.seats, taken_text)
  local status = entry.out and "out"
    or (view.phase == "pass" and entry.pass_chosen and "passed")
    or ""
  local nodes = {
    compose.text(text_util.align_right(tostring(entry.score), score_width) .. " pts", Group.KEY),
    compose.text(" " .. text_util.pad_to_width(taken_text(entry), taken_width)),
    compose.text(
      " " .. text_util.pad_to_width(PLACES[offset_of(update, ctx.me, seat)].name, PLACE_WIDTH),
      Group.MUTED
    ),
  }
  if status ~= "" then
    nodes[#nodes + 1] = compose.text(" " .. status, Group.MUTED)
  end
  return nodes
end

---@type games.GameInput
M.input = {
  kind = PLAY_KIND,
  prompt = "Play, e.g. qs: ",
}

---@type games.GameAction[]
M.actions = {
  {
    key = "p",
    label = "pass cards",
    input = {
      kind = PASS_KIND,
      prompt = "Pass three, e.g. 2c qs ah: ",
    },
    available = function(update)
      return update.view.must_pass > 0
    end,
  },
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  hand_started = function(event)
    return ("hand %d, %s"):format(event.hand_no, passing_text(event.pass))
  end,
  pass_chosen = function(event, talk)
    return ("%s has chosen the cards to pass"):format(talk.name(event.seat))
  end,
  passed = function(event)
    return ("the cards are passed %s"):format(event.pass)
  end,
  played = function(event, talk)
    return ("%s plays %s"):format(talk.name(event.seat), talk.cards({ event.card }))
  end,
  trick_won = function(event, talk)
    return ("%s takes the trick, %s"):format(talk.name(event.seat), points_text(event.points))
  end,
  hand_scored = function(event, talk)
    local score = event.score
    if score.moon then
      return ("hand %d: %s shoots the moon"):format(score.hand_no, talk.name(score.moon))
    end
    local points = vim.tbl_map(function(entry)
      return ("%s +%d"):format(talk.name(entry.seat), entry.points)
    end, score.points)
    return ("hand %d: %s"):format(score.hand_no, table.concat(points, ", "))
  end,
  left = function(event, talk)
    return ("%s left the game; its cards are played for it"):format(talk.name(event.by))
  end,
}

return M
