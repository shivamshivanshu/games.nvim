local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local dice = require("games.ui.dice")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local text_util = require("games.ui.text")

---@class games.LiarsDiceBid
---@field by games.Seat
---@field quantity integer
---@field face games.DieFace

---@class games.LiarsDiceSeatView
---@field seat games.Seat
---@field dice integer
---@field out boolean no dice left, lost or left the game

---@class games.LiarsDiceBidMinimum
---@field face games.DieFace
---@field quantity integer the least the viewer may bid on face; up to total_dice is legal

---@class games.LiarsDiceSeatDice
---@field seat games.Seat
---@field dice games.DieFace[]

---@class games.LiarsDiceReveal
---@field round integer
---@field bid games.LiarsDiceBid
---@field challenger games.Seat
---@field counted integer dice that counted toward the bid, wild ones included
---@field loser games.Seat
---@field cups games.LiarsDiceSeatDice[]

---@class games.LiarsDiceView
---@field round integer
---@field seats games.LiarsDiceSeatView[]
---@field bid? games.LiarsDiceBid nil before the round's opening bid
---@field total_dice integer
---@field my_dice games.DieFace[]
---@field min_bids games.LiarsDiceBidMinimum[] the viewer's own: empty unless it is to act
---@field can_dudo boolean the viewer's own
---@field last_reveal? games.LiarsDiceReveal the previous round's challenge

---@class games.LiarsDiceUpdate: games.GameUpdate
---@field view games.LiarsDiceView

---@class games.LiarsDiceBidEvent: games.Event
---@field kind "bid"
---@field by games.Seat
---@field quantity integer
---@field face games.DieFace

---@class games.LiarsDiceChallengedEvent: games.Event
---@field kind "challenged"
---@field by games.Seat
---@field bid games.LiarsDiceBid
---@field counted integer
---@field loser games.Seat
---@field cups games.LiarsDiceSeatDice[]

---@class games.LiarsDiceOutEvent: games.Event
---@field kind "out"
---@field seat games.Seat

local STARTING_DICE = 5
local NO_DIE_KEY = "none"
local KEY_HL = "GamesKey"
local MUTED_HL = "GamesMuted"
local BID_KIND = "bid"
---Display columns the table panel reserves: "Round 1: carol called dudo on 30 x ⚅", with room for a
---longer name.
local PANEL_WIDTH = 40

---@param n integer
---@return string
local function dice_count(n)
  return n == 1 and "1 die" or ("%d dice"):format(n)
end

---@param talk games.Talk
---@param bid games.LiarsDiceBid|games.LiarsDiceBidEvent
---@return string
local function bid_text(talk, bid)
  return ("%d x %s"):format(bid.quantity, talk.faces({ bid.face }))
end

---@param view games.LiarsDiceView
---@param talk games.Talk
---@return games.KvPair[]
local function table_pairs(view, talk)
  local pairs = {
    { "Round", ("%d, %s in play"):format(view.round, dice_count(view.total_dice)) },
    {
      "Bid",
      view.bid and ("%s by %s"):format(bid_text(talk, view.bid), talk.name(view.bid.by))
        or "none yet",
    },
  }
  if #view.min_bids > 0 then
    local minimums = vim.tbl_map(function(least)
      return ("%dx%s"):format(least.quantity, talk.faces({ least.face }))
    end, view.min_bids)
    pairs[#pairs + 1] = { view.bid and "Raise" or "Open", table.concat(minimums, "  ") }
  end
  return pairs
end

---The previous challenge in words, then every cup it revealed.
---@param reveal games.LiarsDiceReveal|nil
---@param talk games.Talk
---@return games.Node[]
local function reveal_lines(reveal, talk)
  if not reveal then
    return {}
  end
  local lines = {
    compose.text(""),
    compose.text(
      ("Round %d: %s called dudo on %s"):format(
        reveal.round,
        talk.name(reveal.challenger),
        bid_text(talk, reveal.bid)
      ),
      KEY_HL
    ),
    compose.text(
      ("%d counted, %s lost a die"):format(reveal.counted, talk.name(reveal.loser)),
      MUTED_HL
    ),
  }
  for _, cup in ipairs(reveal.cups) do
    local name = text_util.fit_to_width(talk.name(cup.seat), text_util.NAME_WIDTH)
    lines[#lines + 1] = compose.text(name .. " " .. talk.faces(cup.dice))
  end
  return lines
end

local M = {}

---@type string[] keys every liar's dice asset pack draws: the six faces and an empty place in the cup
M.asset_keys = vim.list_extend(dice.keys(), { NO_DIE_KEY })

---Your own dice, with the table and the last reveal beside them.
---@param update games.LiarsDiceUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local talk = describe.talk(update, { art = ctx.art, ambiwidth = ctx.ambiwidth })
  return board({
    grid = {
      rows = 1,
      cols = STARTING_DICE,
      cell = function(_, c)
        local face = view.my_dice[c]
        return { face and dice.key(face) or NO_DIE_KEY }
      end,
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          local nodes = { kv({ pairs = table_pairs(view, talk), key_hl = KEY_HL }) }
          return compose.col(vim.list_extend(nodes, reveal_lines(view.last_reveal, talk)))
        end,
      },
    },
  }, ctx)
end

---Each seat's dice left, after the clock on its header line.
---@param update games.LiarsDiceUpdate
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
  return { compose.text(dice_count(entry.dice), KEY_HL) }
end

---@type games.GameInput
M.input = {
  kind = BID_KIND,
  prompt = "Bid, e.g. 3x5: ",
}

---@type games.GameAction[]
M.actions = {
  {
    key = "d",
    label = "dudo",
    action = { kind = "dudo" },
    available = function(update)
      return update.view.can_dudo
    end,
  },
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  bid = function(event, talk)
    ---@cast event games.LiarsDiceBidEvent
    return ("%s bids %s"):format(talk.name(event.by), bid_text(talk, event))
  end,
  challenged = function(event, talk)
    return ("%s calls dudo on %s: %d counted, %s loses a die"):format(
      talk.name(event.by),
      bid_text(talk, event.bid),
      event.counted,
      talk.name(event.loser)
    )
  end,
  out = function(event, talk)
    return ("%s has no dice left"):format(talk.name(event.seat))
  end,
}

return M
