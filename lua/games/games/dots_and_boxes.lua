local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local cursor = require("games.ui.cursor")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local notation = require("games.ui.notation")
local players = require("games.core.players")
local text = require("games.ui.text")
local theme = require("games.ui.theme")

local Group = highlights.Group

---@alias games.DotsAndBoxesSeat "p1"|"p2"

---An edge: the dot at its left end lying across, or at its top end lying down.
---@class games.DotsAndBoxesLine: games.GridPos
---@field orientation games.Orientation

---@class games.DotsAndBoxesView
---@field across (games.DotsAndBoxesSeat|"")[][] who drew the edge across from each dot, rows of dots from the top; "" while undrawn
---@field down (games.DotsAndBoxesSeat|"")[][] who drew the edge down from each dot
---@field boxes (games.DotsAndBoxesSeat|"")[][] who completed each box; "" while open
---@field scores table<games.DotsAndBoxesSeat, integer> each seat's boxes
---@field last? games.DotsAndBoxesLine the latest edge drawn
---@field legal_edges games.DotsAndBoxesLine[] the viewer's own: the undrawn edges, none unless it is to move

---@class games.DotsAndBoxesUpdate: games.GameUpdate
---@field view games.DotsAndBoxesView

---@class games.DotsAndBoxesDrawnEvent: games.Event
---@field kind "drawn"
---@field by games.DotsAndBoxesSeat
---@field edge games.DotsAndBoxesLine
---@field boxes games.Cell[] the boxes the edge completed
---@field again boolean whether by moves again, having completed a box with the game still on

local DRAW_KIND = "draw"
local ORIENTATION_SUFFIX = { across = "h", down = "v" }
---Display columns of a box one line tall: with its edges about as wide as it is tall on screen.
local BOX_WIDTH = 3
local DOT_KEY, OPEN_KEY, BOX_KEY, BLOCKED_KEY = "dot", "open", "box", "blocked"
local DRAWN_KEY = { across = "across", down = "down" }
local LAST_KEY = { across = "last_across", down = "last_down" }
local AIM_KEY = { across = "aim_across", down = "aim_down" }
local EDGE_HL, DOT_HL, LAST_HL, AIM_HL, BLOCKED_HL =
  Group.LABEL, Group.MUTED, Group.LAST_MOVE, Group.MOVABLE, Group.CHECK
local STATUS_HL, VALUE_HL = Group.KEY, Group.LABEL
---Display columns the panel reserves: its widest line, "at a1h: drawn".
local PANEL_WIDTH = #"at a1h: drawn"

---@param update games.DotsAndBoxesUpdate
---@return integer boxes on a side
local function side(update)
  return #update.view.boxes
end

---@param update games.DotsAndBoxesUpdate
---@return games.OrientedBounds the dots the cursor's edge may start from, lying either way
local function reach(update)
  local n = side(update)
  return { across = { rows = n + 1, cols = n }, down = { rows = n, cols = n + 1 } }
end

---@param update games.GameUpdate
---@return boolean
local function drawing(update)
  return #update.view.legal_edges > 0
end

---@param line games.DotsAndBoxesLine|games.GridCursor
---@return string as typed: the dot's column letter and row number, then h or v, e.g. b2h
local function edge_name(line)
  return text.ALPHABET:sub(line.col, line.col) .. line.row .. ORIENTATION_SUFFIX[line.orientation]
end

---@param line games.DotsAndBoxesLine|games.GridCursor|nil
---@param row integer
---@param col integer
---@param orientation games.Orientation
---@return boolean
local function is_line(line, row, col, orientation)
  return line ~= nil and line.row == row and line.col == col and line.orientation == orientation
end

---@param view games.DotsAndBoxesView
---@param at games.GridCursor
---@return boolean whether the viewer may draw the edge under the cursor
local function legal(view, at)
  return vim.iter(view.legal_edges):any(function(line)
    return is_line(line, at.row, at.col, at.orientation)
  end)
end

---@param update games.DotsAndBoxesUpdate
---@param ctx games.LayoutCtx
---@param orientation games.Orientation
---@return fun(r: integer, c: integer): games.GridCell each edge lying orientation: the cursor's, the
---latest, drawn or open
local function edges_lying(update, ctx, orientation)
  local view = update.view
  local drawers = orientation == cursor.Orientation.ACROSS and view.across or view.down
  local at = drawing(update) and cursor.of(ctx) or nil
  return function(r, c)
    if at and is_line(at, r, c, orientation) then
      return legal(view, at) and { AIM_KEY[orientation], AIM_HL } or { BLOCKED_KEY, BLOCKED_HL }
    end
    if drawers[r][c] == "" then
      return { OPEN_KEY }
    end
    if is_line(view.last, r, c, orientation) then
      return { LAST_KEY[orientation], LAST_HL }
    end
    return { DRAWN_KEY[orientation], EDGE_HL }
  end
end

---A box shows its owner's glyph in the colour the header gives that player.
---@param update games.DotsAndBoxesUpdate
---@param owner games.DotsAndBoxesSeat|""
---@return games.GridCell
local function box_cell(update, owner)
  if owner == "" then
    return { BOX_KEY }
  end
  return { owner, theme.seat_hl_of(update, owner) }
end

---@param update games.DotsAndBoxesUpdate
---@param ctx games.LayoutCtx
---@return games.Node what the viewer does now, the edge under its cursor, and the latest edge
local function panel(update, ctx)
  local view = update.view
  local lines = {}
  if update.result then
    lines[1] = compose.text("game over", STATUS_HL)
  elseif players.is_to_act(update, ctx.me) then
    local at = cursor.of(ctx)
    lines[1] = compose.text("your move", STATUS_HL)
    local where = "at " .. edge_name(at)
    lines[2] = compose.text(legal(view, at) and where or (where .. ": drawn"), VALUE_HL)
  else
    lines[1] = compose.text("their move", STATUS_HL)
  end
  if view.last then
    lines[#lines + 1] = compose.text("last " .. edge_name(view.last), VALUE_HL)
  end
  return compose.col(lines)
end

local M = {}

---@type string[] keys every dots_and_boxes asset pack draws: a dot; an edge open, drawn, drawn
---last and under the cursor either way, or blocked under it; an open box and each seat's box
M.asset_keys = {
  DOT_KEY,
  OPEN_KEY,
  DRAWN_KEY.across,
  DRAWN_KEY.down,
  LAST_KEY.across,
  LAST_KEY.down,
  AIM_KEY.across,
  AIM_KEY.down,
  BLOCKED_KEY,
  BOX_KEY,
  "p1",
  "p2",
}

---@param update games.DotsAndBoxesUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local n = side(update)
  return board({
    grid = {
      rows = n,
      cols = n,
      cell_width = BOX_WIDTH,
      labels = { cols = notation.letters(n + 1), rows = notation.numbers(n + 1) },
      cell = function(r, c)
        return box_cell(update, view.boxes[r][c])
      end,
      edges = {
        across = edges_lying(update, ctx, cursor.Orientation.ACROSS),
        down = edges_lying(update, ctx, cursor.Orientation.DOWN),
        dot = function()
          return { DOT_KEY, DOT_HL }
        end,
      },
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return panel(update, ctx)
        end,
      },
    },
  }, ctx)
end

---Each player's glyph and boxes, after the clock on its header line.
---@param update games.DotsAndBoxesUpdate
---@param seat games.DotsAndBoxesSeat
---@param ctx games.ArtLayoutCtx
---@return games.TextNode[]
function M.player_extra(update, seat, ctx)
  local boxes = update.view.scores[seat] or 0
  local score = ("%s %d box%s"):format(
    art.glyph(ctx.art, seat, ctx.ambiwidth),
    boxes,
    boxes == 1 and "" or "es"
  )
  return { compose.text(score, theme.seat_hl_of(update, seat)) }
end

---@type games.GameAction[]
M.actions = vim.list_extend(cursor.moves(reach, drawing), {
  cursor.rotate(drawing, reach),
  {
    key = "<CR>",
    label = DRAW_KIND,
    act = function(update, view)
      local at = cursor.of(view)
      return legal(update.view, at) and { kind = DRAW_KIND, edge = edge_name(at) } or nil
    end,
  },
})

---@type table<games.EventKind, games.Describer>
M.describers = {
  drawn = function(event, talk)
    ---@cast event games.DotsAndBoxesDrawnEvent
    local drew = ("%s drew %s"):format(talk.name(event.by), edge_name(event.edge))
    local completed = #event.boxes
    if completed == 0 then
      return drew
    end
    local again = event.again and ", and moves again" or ""
    return ("%s, completing %d box%s%s"):format(
      drew,
      completed,
      completed == 1 and "" or "es",
      again
    )
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function entry(cell, meaning)
    return { sample = { grid.sample(ctx, cell) }, meaning = meaning }
  end
  local across = cursor.Orientation.ACROSS
  return {
    entry({ DOT_KEY, DOT_HL }, "a dot"),
    entry({ DRAWN_KEY[across], EDGE_HL }, "a line drawn"),
    entry({ LAST_KEY[across], LAST_HL }, "the last line drawn, also named in the panel"),
    entry({ AIM_KEY[across], AIM_HL }, "the cursor, on a line you may draw"),
    entry({ BLOCKED_KEY, BLOCKED_HL }, "the cursor, on a line already drawn"),
    {
      sample = {
        grid.sample(ctx, { "p1", theme.seat_hl(1) }),
        compose.text(" "),
        grid.sample(ctx, { "p2", theme.seat_hl(2) }),
      },
      meaning = "a completed box, in its owner's colour in the header",
    },
  }
end

return M
