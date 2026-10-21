local actions = require("games.ui.actions")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local dice = require("games.ui.dice")
local grid = require("games.ui.widgets.grid")
local notation = require("games.ui.notation")
local players = require("games.core.players")
local text_util = require("games.ui.text")

---@alias games.YahtzeeBox "ones"|"twos"|"threes"|"fours"|"fives"|"sixes"|"three_of_a_kind"|"four_of_a_kind"|"full_house"|"small_straight"|"large_straight"|"yahtzee"|"chance"

---@class games.YahtzeeCard
---@field seat games.Seat
---@field scores table<games.YahtzeeBox, integer> the filled boxes alone
---@field upper_total integer
---@field upper_bonus integer 35 once upper_total reaches 63
---@field yahtzee_bonus integer
---@field total integer
---@field out boolean the seat left the game; its card stays

---@class games.YahtzeeOption
---@field box games.YahtzeeBox
---@field score integer what the box would get now

---@class games.YahtzeeView
---@field round integer
---@field dice games.DieFace[] empty before the turn's first roll
---@field held boolean[] one per die
---@field rolls_left integer
---@field cards games.YahtzeeCard[] in seat order
---@field can_roll boolean the viewer's own
---@field can_hold boolean the viewer's own
---@field options games.YahtzeeOption[] the viewer's own: the boxes it may score now

---@class games.YahtzeeUpdate: games.GameUpdate
---@field view games.YahtzeeView

---@class games.YahtzeeRolledEvent: games.Event
---@field kind "rolled"
---@field by games.Seat
---@field dice games.DieFace[] all five after the roll

---@class games.YahtzeeScoredEvent: games.Event
---@field kind "scored"
---@field by games.Seat
---@field box games.YahtzeeBox
---@field score integer
---@field yahtzee_bonus integer 0 or 100

---@class (private) games.YahtzeeBoxRow
---@field box games.YahtzeeBox
---@field key string scores the box
---@field label string

---@class (private) games.YahtzeeTotalRow
---@field label string
---@field field "upper_total"|"upper_bonus"|"yahtzee_bonus"|"total"

---@alias games.YahtzeeRow games.YahtzeeBoxRow|games.YahtzeeTotalRow

local DICE_COUNT = 5
---@type integer[] each die by its place, as the hold action numbers it
local DICE = vim.fn.range(1, DICE_COUNT)
---@type integer[]
local NO_DICE = {}
local DICE_ROW, HOLD_ROW = 1, 2
local NO_DIE_KEY, HELD_KEY, FREE_KEY = "none", "held", "free"
local HELD_HL = "GamesMovable"
local KEY_HL = "GamesKey"
local LABEL_HL = "GamesLabel"
local MUTED_HL = "GamesMuted"
local KEY_WIDTH = 2
local LABEL_WIDTH = 16
---Display columns of each seat's score column, its name cut to fit with a blank before it.
local SEAT_WIDTH = 7
local NOW_WIDTH = 5
local NOW_HEADING = "now"

---The scorecard in order, each box with the key that scores it; `m` is the shell's.
---@type games.YahtzeeRow[]
local ROWS = {
  { box = "ones", key = "a", label = "Ones" },
  { box = "twos", key = "b", label = "Twos" },
  { box = "threes", key = "c", label = "Threes" },
  { box = "fours", key = "d", label = "Fours" },
  { box = "fives", key = "e", label = "Fives" },
  { box = "sixes", key = "f", label = "Sixes" },
  { label = "Upper", field = "upper_total" },
  { label = "Bonus at 63", field = "upper_bonus" },
  { box = "three_of_a_kind", key = "g", label = "3 of a kind" },
  { box = "four_of_a_kind", key = "h", label = "4 of a kind" },
  { box = "full_house", key = "i", label = "Full house" },
  { box = "small_straight", key = "j", label = "Small straight" },
  { box = "large_straight", key = "k", label = "Large straight" },
  { box = "yahtzee", key = "l", label = "Yahtzee" },
  { box = "chance", key = "n", label = "Chance" },
  { label = "Yahtzee bonus", field = "yahtzee_bonus" },
  { label = "Total", field = "total" },
}

---@type games.YahtzeeBoxRow[]
local BOX_ROWS = vim.tbl_filter(function(row)
  return row.box ~= nil
end, ROWS)

---@type table<games.YahtzeeBox, string>
local BOX_LABEL = vim.iter(BOX_ROWS):fold({}, function(labels, row)
  labels[row.box] = row.label:lower()
  return labels
end)

---@param view games.YahtzeeView
---@param row integer
---@param col integer
---@return games.GridCell
local function cell_for(view, row, col)
  if row == HOLD_ROW then
    return { view.held[col] and HELD_KEY or FREE_KEY }
  end
  local face = view.dice[col]
  return { face and dice.key(face) or NO_DIE_KEY }
end

---@param view games.YahtzeeView
---@return games.GridMark[]
local function held_marks(view)
  local marks = {}
  for col = 1, #view.dice do
    marks[#marks + 1] = view.held[col] and grid.mark(DICE_ROW, col, HELD_HL) or nil
  end
  return marks
end

---@param view games.YahtzeeView
---@param box games.YahtzeeBox
---@return games.YahtzeeOption|nil
local function option_for(view, box)
  return vim.iter(view.options):find(function(option)
    return option.box == box
  end)
end

---@param card games.YahtzeeCard
---@param row games.YahtzeeRow
---@return string
local function card_value(card, row)
  local value = row.box and card.scores[row.box] or card[row.field]
  return value and tostring(value) or "-"
end

---@param update games.YahtzeeUpdate
---@return games.Node
local function heading(update)
  local parts = { compose.text(text_util.spaces(KEY_WIDTH + LABEL_WIDTH)) }
  for _, card in ipairs(update.view.cards) do
    local name = vim.fn.strcharpart(players.name_of_seat(update, card.seat), 0, SEAT_WIDTH - 1)
    parts[#parts + 1] = compose.text(text_util.align_right(name, SEAT_WIDTH), LABEL_HL)
  end
  parts[#parts + 1] = compose.text(text_util.align_right(NOW_HEADING, NOW_WIDTH), LABEL_HL)
  return compose.row(parts)
end

---One scorecard line: an offered box's key and what it would score now, then every seat's entry.
---@param view games.YahtzeeView
---@param row games.YahtzeeRow
---@return games.Node
local function scorecard_line(view, row)
  local option = row.box and option_for(view, row.box)
  local parts = {
    compose.text(text_util.pad_to_width(option and row.key or "", KEY_WIDTH), KEY_HL),
    compose.text(text_util.pad_to_width(row.label, LABEL_WIDTH), row.box and LABEL_HL or KEY_HL),
  }
  for _, card in ipairs(view.cards) do
    parts[#parts + 1] = compose.text(text_util.align_right(card_value(card, row), SEAT_WIDTH))
  end
  local now = option and tostring(option.score) or ""
  parts[#parts + 1] = compose.text(text_util.align_right(now, NOW_WIDTH), KEY_HL)
  return compose.row(parts)
end

---@param update games.YahtzeeUpdate
---@return games.Node
local function turn_line(update)
  local view = update.view
  local round = ("Round %d of %d"):format(view.round, #BOX_ROWS)
  if not update.result then
    round = round
      .. (", %d roll%s left"):format(view.rolls_left, view.rolls_left == 1 and "" or "s")
  end
  return compose.text(round, MUTED_HL)
end

---@param update games.YahtzeeUpdate
---@return games.Node
local function scorecard(update)
  local view = update.view
  local lines = { turn_line(update), heading(update) }
  for _, row in ipairs(ROWS) do
    lines[#lines + 1] = scorecard_line(view, row)
  end
  return compose.col(lines)
end

local M = {}

---@type string[] keys every yahtzee asset pack draws: the six faces, a die not yet rolled, and the
---marks under a held die and a free one
M.asset_keys = vim.list_extend(dice.keys(), { NO_DIE_KEY, HELD_KEY, FREE_KEY })

---@param update games.YahtzeeUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = HOLD_ROW,
      cols = DICE_COUNT,
      labels = { cols = notation.numbers(DICE_COUNT), rows = { "", "" } },
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = held_marks(view),
    },
    panels = {
      {
        width = KEY_WIDTH + LABEL_WIDTH + #view.cards * SEAT_WIDTH + NOW_WIDTH,
        node = function()
          return scorecard(update)
        end,
      },
    },
  }, ctx)
end

---Each seat's total, after the clock on its header line, and whether it left.
---@param update games.YahtzeeUpdate
---@param seat games.Seat
---@return games.TextNode[]|nil
function M.player_extra(update, seat)
  local card = players.entry_of_seat(update.view.cards, seat)
  if not card then
    return nil
  end
  local nodes = { compose.text(("total %d"):format(card.total), LABEL_HL) }
  if card.out then
    nodes[#nodes + 1] = compose.text(" out", MUTED_HL)
  end
  return nodes
end

---@type games.GameAction[]
M.actions = vim.list_extend(
  {
    {
      key = "r",
      label = "roll",
      action = { kind = "roll" },
      available = function(update)
        return update.view.can_roll
      end,
    },
  },
  vim.list_extend(
    actions.per_value({
      kind = "hold",
      field = "die",
      values = DICE,
      legal = function(view)
        return view.can_hold and DICE or NO_DICE
      end,
    }),
    vim.tbl_map(function(row)
      return {
        key = row.key,
        label = "score",
        action = { kind = "score", box = row.box },
        available = function(update)
          return option_for(update.view, row.box) ~= nil
        end,
      }
    end, BOX_ROWS)
  )
)

---@type table<games.EventKind, games.Describer>
M.describers = {
  rolled = function(event, talk)
    ---@cast event games.YahtzeeRolledEvent
    return ("%s rolled %s"):format(talk.name(event.by), talk.faces(event.dice))
  end,
  scored = function(event, talk)
    ---@cast event games.YahtzeeScoredEvent
    local bonus = event.yahtzee_bonus > 0
        and (" and a %d Yahtzee bonus"):format(event.yahtzee_bonus)
      or ""
    return ("%s scored %d in %s%s"):format(
      talk.name(event.by),
      event.score,
      BOX_LABEL[event.box],
      bonus
    )
  end,
}

return M
