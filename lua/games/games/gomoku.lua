local discs = require("games.ui.discs")

---@class games.GomokuView: games.DiscView
---@field cells (games.DiscSeat|"")[][] 15 rows of 15 from the top; "" for an empty cell
---@field last? games.DiscCell where the latest stone went
---@field winning_lines games.DiscCell[][] every run of five or more through the last stone, whole
---@field legal_cells games.DiscCell[] the viewer's own: the empty cells, none unless it is to move

---@class games.GomokuUpdate: games.GameUpdate
---@field view games.GomokuView

---@class games.GomokuPlacedEvent: games.Event
---@field kind "placed"
---@field by games.DiscSeat
---@field cell games.DiscCell

local SIZE = 15

local M = {}

---@type string[] keys every gomoku asset pack draws
M.asset_keys = discs.asset_keys()

---Every empty cell is legal, so the board marks none and shows only the stones.
---@param update games.GomokuUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return discs.layout(update, ctx, { size = SIZE, winning_lines = update.view.winning_lines })
end

M.input = discs.input("h8")

---@param event games.Event
---@param update games.GomokuUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "placed" then
    return nil
  end
  ---@cast event games.GomokuPlacedEvent
  return ("%s placed %s"):format(discs.who(update, event.by), discs.cell_name(event.cell))
end

return M
