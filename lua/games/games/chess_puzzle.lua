local chess = require("games.games.chess")
local compose = require("games.ui.compose")
local kv = require("games.ui.widgets.kv")
local movelist = require("games.ui.widgets.movelist")
local players = require("games.core.players")

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
  local flip = (solver_side(update) == BLACK) ~= ctx.flipped
  return chess.board(update.view, ctx, flip, {
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
  {
    key = "F",
    label = "flip",
    local_fn = function(view)
      return vim.tbl_extend("force", view, { flipped = not view.flipped })
    end,
  },
}

---@param event games.Event
---@param update games.ChessPuzzleUpdate
---@return string|nil
function M.describe_event(event, update)
  local who = players.name_of_seat(update, event.by)
  if event.kind == "moved" then
    return ("%s played %s"):format(who, event.san)
  elseif event.kind == "replied" then
    return ("%s replied %s"):format(replying_side(update), event.san)
  elseif event.kind == "missed" then
    return ("%s tried %s, not the move"):format(who, event.san)
  elseif event.kind == "revealed" then
    return ("the move was %s"):format(event.san)
  end
  return nil
end

return M
