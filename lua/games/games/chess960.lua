local chess = require("games.games.chess")
local kv = require("games.ui.widgets.kv")

---@class games.Chess960View: games.ChessView
---@field start_position integer the start position's number, 0 to 959, in Scharnagl's numbering

---@class games.Chess960Update: games.ChessUpdate
---@field view games.Chess960View

---@param update games.Chess960Update
---@return games.Node
local function start_panel(update)
  return kv.panel({ { "position", tostring(update.view.start_position) } })
end

---@param update games.Chess960Update
---@param ctx games.ArtLayoutCtx
---@return games.Node
local function layout(update, ctx)
  return chess.board(update.view, ctx, ctx.flip, {
    panels = {
      {
        node = function()
          return start_panel(update)
        end,
      },
    },
  })
end

return vim.tbl_extend("force", chess, { layout = layout })
