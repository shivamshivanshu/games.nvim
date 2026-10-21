local compose = require("games.ui.compose")
local discs = require("games.ui.discs")

---@class games.ReversiDiscCount
---@field black integer
---@field white integer

---@class games.ReversiView: games.DiscView
---@field cells (games.DiscSeat|"")[][] 8 rows of 8 from the top; "" for an empty cell
---@field last? games.Cell where the latest disc went
---@field legal_cells games.Cell[] the viewer's own: the cells it may play, none unless it is to move
---@field discs games.ReversiDiscCount each seat's discs on the board

---@class games.ReversiUpdate: games.GameUpdate
---@field view games.ReversiView

---@class games.ReversiPlacedEvent: games.Event
---@field kind "placed"
---@field by games.DiscSeat
---@field cell games.Cell
---@field flipped games.Cell[] the discs the move turned over

---@class games.ReversiPassedEvent: games.Event
---@field kind "passed"
---@field by games.DiscSeat the seat that had no move

local COUNT_HL = "GamesLabel"

local M = {}

---@type string[] keys every reversi asset pack draws
M.asset_keys = vim.list_extend(discs.asset_keys(), { discs.LEGAL_KEY })

---@param update games.ReversiUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return discs.layout(update, ctx, { size = #update.view.cells, legal = update.view.legal_cells })
end

M.input = discs.input("d3")

---Each player's disc count, after the clock on its header line.
---@param update games.ReversiUpdate
---@param seat games.DiscSeat
---@return games.TextNode[]
function M.player_extra(update, seat)
  local count = update.view.discs[seat]
  return { compose.text(("%d disc%s"):format(count, count == 1 and "" or "s"), COUNT_HL) }
end

---@type table<games.EventKind, games.Describer>
M.describers = {
  placed = function(event, talk)
    ---@cast event games.ReversiPlacedEvent
    return ("%s placed %s, flipping %d"):format(
      talk.name(event.by),
      discs.cell_name(event.cell),
      #event.flipped
    )
  end,
  passed = function(event, talk)
    ---@cast event games.ReversiPassedEvent
    return ("%s has no move and passes"):format(talk.name(event.by))
  end,
}

return M
