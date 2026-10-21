local chess = require("games.games.chess")
local kv = require("games.ui.widgets.kv")

---@class games.Chess960View: games.ChessView
---@field start_position integer the start position's number, 0 to 959, in Scharnagl's numbering
---@field start_fen string the start position, which the move list numbers on from

---@class games.Chess960Update: games.ChessUpdate
---@field view games.Chess960View

---@param update games.Chess960Update
---@return games.Node
local function start_panel(update)
  return kv.panel({ { "position", tostring(update.view.start_position) } })
end

local M = {}

M.asset_keys = chess.asset_keys
M.player_extra = chess.player_extra
M.input = chess.input
M.actions = chess.actions
M.describers = chess.describers

---@param update games.Chess960Update
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return chess.board(update.view, ctx, chess.black_at_bottom(update, ctx), {
    first_ply = chess.first_ply(update.view.start_fen),
    panels = {
      {
        node = function()
          return start_panel(update)
        end,
      },
    },
  })
end

return M
