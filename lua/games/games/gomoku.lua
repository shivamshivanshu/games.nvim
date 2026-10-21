local discs = require("games.ui.discs")

---@class games.GomokuView: games.DiscView
---@field cells (games.DiscSeat|"")[][] 15 rows of 15 from the top; "" for an empty cell
---@field last? games.Cell where the latest stone went
---@field winning_lines games.Cell[][] every run of five or more through the last stone, whole
---@field legal_cells games.Cell[] the viewer's own: the empty cells, none unless it is to move

---@class games.GomokuUpdate: games.GameUpdate
---@field view games.GomokuView

---@class games.GomokuPlacedEvent: games.Event
---@field kind "placed"
---@field by games.DiscSeat
---@field cell games.Cell

local M = {}

---@type string[] keys every gomoku asset pack draws
M.asset_keys = discs.asset_keys()

---Every empty cell is legal, so the board marks none and shows only the stones.
---@param update games.GomokuUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return discs.layout(
    update,
    ctx,
    { size = #update.view.cells, winning_lines = update.view.winning_lines }
  )
end

M.input = discs.input("h8")

---@type table<games.EventKind, games.Describer>
M.describers = {
  placed = function(event, talk)
    ---@cast event games.GomokuPlacedEvent
    return ("%s placed %s"):format(talk.name(event.by), discs.cell_name(event.cell))
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  return discs.legend(ctx, { discs.MarkKind.WINNING_LINE })
end

return M
