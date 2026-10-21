local chess = require("games.games.chess")
local compose = require("games.ui.compose")
local kv = require("games.ui.widgets.kv")
local movelist = require("games.ui.widgets.movelist")

---@alias games.ChessPuzzleMode "puzzle"|"study"
---@alias games.ChessPuzzleSeat "solver"

---@class games.ChessPuzzleView: games.ChessBoardView
---@field legal_moves string[] UCI while the game runs; empty once it is over
---@field mode games.ChessPuzzleMode
---@field misses integer missed and revealed moves
---@field remaining integer the solver's moves left in the line
---@field solution string[] the whole line in SAN once the game is over; empty until then

---@class games.ChessPuzzleUpdate: games.GameUpdate
---@field view games.ChessPuzzleView

---@class games.ChessPuzzleMovedEvent: games.Event
---@field kind "moved"
---@field by games.ChessPuzzleSeat
---@field san string

---@class games.ChessPuzzleRepliedEvent: games.Event
---@field kind "replied"
---@field by games.ChessPuzzleSeat
---@field san string

---@class games.ChessPuzzleMissedEvent: games.Event
---@field kind "missed"
---@field by games.ChessPuzzleSeat
---@field san string

---@class games.ChessPuzzleRevealedEvent: games.Event
---@field kind "revealed"
---@field by games.ChessPuzzleSeat
---@field san string

---@type games.ChessPuzzleMode
local STUDY = "study"
---@type games.ChessPuzzleSeat
local SOLVER = "solver"
local BLACK = "black"
local WHITE = "white"
local REVEAL_KIND = "reveal"
---Lines the status panel keeps above the solution: its heading and the blank line before it.
local SOLUTION_HEADING_LINES = 2
local STATUS_WIDTH = 20

---@param update games.ChessPuzzleUpdate
---@return string
local function solver_side(update)
  return update.options.player
end

---@param update games.ChessPuzzleUpdate
---@return string
local function replying_side(update)
  return solver_side(update) == BLACK and WHITE or BLACK
end

---@param update games.ChessPuzzleUpdate
---@return games.KvPair[]
local function status_rows(update)
  local view = update.view
  return {
    { "mode", view.mode },
    { "you play", solver_side(update) },
    { "moves left", tostring(view.remaining) },
    { "misses", tostring(view.misses) },
  }
end

---@param update games.ChessPuzzleUpdate
---@param max_height integer
---@return games.Node
local function status_panel(update, max_height)
  local rows = status_rows(update)
  local status = kv.panel(rows)
  local solution = update.view.solution
  if #solution == 0 then
    return status
  end
  return compose.col({
    status,
    compose.text(""),
    compose.text("solution", "GamesKey"),
    movelist({
      items = solution,
      columns = 2,
      numbered = true,
      offset = chess.first_ply(update.options.fen),
      max_lines = math.max(1, max_height - #rows - SOLUTION_HEADING_LINES),
    }),
  })
end

local M = {}

M.asset_keys = chess.asset_keys

---@param update games.ChessPuzzleUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return chess.board(update.view, ctx, ctx.flip, {
    first_ply = chess.first_ply(update.options.fen),
    panels = {
      {
        width = STATUS_WIDTH,
        node = function(max_height)
          return status_panel(update, max_height)
        end,
      },
    },
  })
end

M.input = chess.input

---@type games.GameAction[]
M.actions = {
  {
    key = "n",
    label = "reveal",
    action = { kind = REVEAL_KIND },
    available = function(update)
      return update.view.mode == STUDY
    end,
  },
}

---@type games.Flippable
M.flippable = {
  bottom_seat = function(update)
    return solver_side(update --[[@as games.ChessPuzzleUpdate]]) == BLACK and SOLVER or nil
  end,
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  moved = function(event, talk)
    ---@cast event games.ChessPuzzleMovedEvent
    return ("%s played %s"):format(talk.name(event.by), event.san)
  end,
  replied = function(event, talk)
    ---@cast event games.ChessPuzzleRepliedEvent
    return ("%s replied %s"):format(
      replying_side(talk.update --[[@as games.ChessPuzzleUpdate]]),
      event.san
    )
  end,
  missed = function(event, talk)
    ---@cast event games.ChessPuzzleMissedEvent
    return ("%s tried %s, not the move"):format(talk.name(event.by), event.san)
  end,
  revealed = function(event)
    ---@cast event games.ChessPuzzleRevealedEvent
    return ("the move was %s"):format(event.san)
  end,
}

return M
