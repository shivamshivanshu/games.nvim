local board = require("games.ui.widgets.board")
local cards = require("games.ui.cards")
local compose = require("games.ui.compose")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local text_util = require("games.ui.text")

---@alias games.HeartsPass "left"|"right"|"across"|"hold"
---@alias games.HeartsPhase "pass"|"play"

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
---@field passed games.CardKey[] the viewer's own: the cards it chose to pass this hand
---@field received games.CardKey[] the viewer's own: the cards passed to it this hand
---@field last_hand? games.HeartsHandScore

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

---@class games.HeartsHandScoredEvent: games.Event
---@field kind "hand_scored"
---@field by games.Seat
---@field score games.HeartsHandScore

local GLYPH_WIDTH = 2
local EMPTY_KEY = "empty"
local BLANK_KEY = "blank"
local CARD_HL = "GamesSquareLight"
local KEY_HL = "GamesKey"
local MUTED_HL = "GamesMuted"
local PLAY_KIND = "play"
local PASS_KIND = "pass"
local HOLD = "hold"
local SEATS = 4
---Display columns the table panel reserves: a full hand of 13 cards on one line.
local PANEL_WIDTH = 38
---Display columns a user name may take in the last-hand lines.
local NAME_WIDTH = 10

---Where each seat sits round the trick, by its offset clockwise from the viewer, who sits at
---the bottom: the next seat is on its left.
---@type {row: integer, col: integer, name: string}[]
local PLACES = {
  [0] = { row = 3, col = 2, name = "" },
  { row = 2, col = 1, name = "left" },
  { row = 1, col = 2, name = "across" },
  { row = 2, col = 3, name = "right" },
}
---@type table<games.HeartsPass, integer> each pass direction's offset clockwise
local PASS_OFFSETS = { left = 1, across = 2, right = 3 }
local PLACE_WIDTH = text_util.max_display_width(vim.tbl_map(function(place)
  return place.name
end, vim.tbl_values(PLACES)))

---@param update games.HeartsUpdate
---@param seat games.Seat
---@return string
local function name_of(update, seat)
  return players.user_of_seat(update, seat) or seat
end

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
    cells[place.row][place.col] = card and { card, cards.hl(card) } or { EMPTY_KEY, MUTED_HL }
    if card then
      marks[#marks + 1] = { row = place.row, col = place.col, hl = CARD_HL }
    end
  end
  return cells, marks
end

---@class (private) games.HeartsTalk how text names seats and cards
---@field name fun(seat: games.Seat): string
---@field cards fun(cards: games.CardKey[]): string

---@param update games.HeartsUpdate
---@param ctx games.LayoutCtx|games.DescribeCtx
---@return games.HeartsTalk
local function talk_for(update, ctx)
  local set = assert(ctx.art, "hearts draws from an asset pack")
  return {
    name = function(seat)
      return name_of(update, seat)
    end,
    cards = function(shown)
      return cards.text(set, shown, ctx.ambiwidth)
    end,
  }
end

---@param view games.HeartsView
---@param talk games.HeartsTalk
---@return string|nil what the viewer is to do now
local function hint(view, talk)
  if view.must_pass > 0 and view.pass_to then
    return ("pass %d cards to %s"):format(view.must_pass, talk.name(view.pass_to))
  end
  if view.phase == "pass" and #view.passed > 0 then
    return "wait for the others to pass"
  end
  if #view.legal > 0 then
    return #view.trick == 0 and "lead a card" or "play a card"
  end
  return nil
end

---@param update games.HeartsUpdate
---@param me games.UserID
---@param talk games.HeartsTalk
---@return games.KvPair[]
local function table_pairs(update, me, talk)
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
  if #view.received > 0 then
    local giver = seat_at(update, me, SEATS - PASS_OFFSETS[view.pass])
    pairs[#pairs + 1] =
      { "Received", ("%s from %s"):format(talk.cards(view.received), talk.name(giver)) }
  end
  local now = hint(view, talk)
  if now then
    pairs[#pairs + 1] = { "Now", now }
  end
  return pairs
end

---@param view games.HeartsView
---@param ctx games.LayoutCtx
---@return games.Node[]
local function hand_lines(view, ctx)
  if #view.my_hand == 0 then
    return {}
  end
  local marked = {}
  for _, card in ipairs(view.legal) do
    marked[card] = true
  end
  local lines = { compose.text(""), compose.text("Your hand", KEY_HL) }
  return vim.list_extend(
    lines,
    cards.hand({
      set = assert(ctx.art, "hearts draws from an asset pack"),
      cards = view.my_hand,
      marked = marked,
      glyph_width = GLYPH_WIDTH,
      width = PANEL_WIDTH,
      ambiwidth = ctx.ambiwidth,
    })
  )
end

---@param last games.HeartsHandScore|nil
---@param talk games.HeartsTalk
---@return games.Node[]
local function last_hand_lines(last, talk)
  if not last then
    return {}
  end
  local lines = { compose.text(""), compose.text(("Last hand %d"):format(last.hand_no), KEY_HL) }
  for _, entry in ipairs(last.points) do
    local name =
      text_util.pad_to_width(vim.fn.strcharpart(talk.name(entry.seat), 0, NAME_WIDTH), NAME_WIDTH)
    lines[#lines + 1] = compose.text(("%s +%d"):format(name, entry.points))
  end
  if last.moon then
    lines[#lines + 1] = compose.text(("%s shot the moon"):format(talk.name(last.moon)), MUTED_HL)
  end
  return lines
end

local M = {}

---@type string[] keys every hearts asset pack draws: the 52 cards, an empty place in the trick
---and a blank
M.asset_keys = vim.list_extend(cards.keys(), { EMPTY_KEY, BLANK_KEY })

M.glyph_width = GLYPH_WIDTH

---The trick round the table, you at the bottom and the seat on your left on the left, with the
---table, your hand, the cards you may play marked, and the last hand's scores in the panel.
---@param update games.HeartsUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local talk = talk_for(update, ctx)
  local cells, marks = trick_cells(update, ctx.me)
  return board({
    grid = {
      rows = 3,
      cols = 3,
      shade = "plain",
      cell_width = GLYPH_WIDTH,
      cell = function(r, c)
        return cells[r][c] or { BLANK_KEY }
      end,
      marks = marks,
    },
    art = ctx.art,
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          local nodes = { kv({ pairs = table_pairs(update, ctx.me, talk), key_hl = KEY_HL }) }
          vim.list_extend(nodes, hand_lines(view, ctx))
          return compose.col(vim.list_extend(nodes, last_hand_lines(view.last_hand, talk)))
        end,
      },
    },
  }, ctx)
end

---@param view games.HeartsView
---@param seat games.Seat
---@return games.HeartsSeatView|nil
local function seat_view(view, seat)
  return vim.iter(view.seats):find(function(entry)
    return entry.seat == seat
  end)
end

---@param seats games.HeartsSeatView[]
---@param field fun(seat: games.HeartsSeatView): string
---@return integer
local function widest(seats, field)
  return text_util.max_display_width(vim.tbl_map(field, seats))
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
---@param ctx games.LayoutCtx
---@return games.TextNode[]|nil
function M.player_extra(update, seat, ctx)
  local view = update.view
  local entry = seat_view(view, seat)
  if not entry then
    return nil
  end
  local score_width = widest(view.seats, function(s)
    return tostring(s.score)
  end)
  local taken_width = widest(view.seats, taken_text)
  local status = entry.out and "out"
    or (view.phase == "pass" and entry.pass_chosen and "passed")
    or ""
  local nodes = {
    compose.text(text_util.align_right(tostring(entry.score), score_width) .. " pts", KEY_HL),
    compose.text(" " .. text_util.pad_to_width(taken_text(entry), taken_width)),
    compose.text(
      " " .. text_util.pad_to_width(PLACES[offset_of(update, ctx.me, seat)].name, PLACE_WIDTH),
      MUTED_HL
    ),
  }
  if status ~= "" then
    nodes[#nodes + 1] = compose.text(" " .. status, MUTED_HL)
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

---@type table<string, fun(event: table, talk: games.HeartsTalk): string>
local DESCRIBERS = {
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

---@param event games.Event
---@param update games.HeartsUpdate
---@param ctx games.DescribeCtx with art, which a module with asset_keys always gets
---@return string|nil
function M.describe_event(event, update, ctx)
  local describer = DESCRIBERS[event.kind]
  return describer and describer(event, talk_for(update, ctx))
end

return M
