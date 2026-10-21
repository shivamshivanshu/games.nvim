local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local paint = require("games.ui.paint")
local players = require("games.core.players")
local text_util = require("games.ui.text")
local theme = require("games.ui.theme")

---Where the header draws each player's clock, which a clock tick rewrites in place. Clock text is
---ASCII, so its bytes are its display columns.
---@class games.ClockColumn
---@field cols integer[] byte column of each player's clock, by header line
---@field width integer columns of the widest clock, to which every clock is padded

---@class (private) games.ClockCell
---@field text string
---@field hl? games.Hl

local LOW_CLOCK_MS = 10000
local LOW_CLOCK_HL = "GamesClockLow"
local HEADER_COLUMN_GAP = 2

local M = {}

---@param update games.GameUpdate
---@param user games.UserID
---@param me games.UserID
---@return games.Node
local function turn_marker(update, user, me)
  if not players.is_to_act(update, user) then
    return compose.text("")
  end
  if user == me then
    return compose.row({ compose.text("← "), compose.text(" your move ", "GamesYourTurn") })
  end
  return compose.text("← to move", "GamesMuted")
end

---@param update games.GameUpdate with a clock
---@param elapsed_ms integer
---@return games.ClockCell[] by player
function M.clock_cells(update, elapsed_ms)
  return vim.tbl_map(function(reading)
    return {
      text = reading.text,
      hl = reading.remaining_ms < LOW_CLOCK_MS and LOW_CLOCK_HL or nil,
    }
  end, describe.clock_readings(update, elapsed_ms))
end

---@param cells games.ClockCell[]
---@return integer
function M.widest_clock(cells)
  return vim.iter(cells):fold(0, function(width, cell)
    return math.max(width, #cell.text)
  end)
end

---@param update games.GameUpdate
---@param me games.UserID
---@return games.Node|nil
local function status_banner(update, me)
  if update.result then
    local hl = theme.outcome_hl(describe.outcome(update.result, me))
    return compose.text(describe.result_text(update.result), hl)
  end
  if update.draw_agreed and #update.draw_agreed > 0 then
    return compose.text("Draw offered by " .. table.concat(update.draw_agreed, ", "))
  end
end

---Each player's extra segments, one line per player; nil when the game has none for any player.
---@param update games.GameUpdate
---@param module games.GameModule
---@param ctx games.LayoutCtx
---@return games.Node|nil
function M.extras_column(update, module, ctx)
  local player_extra = module.player_extra
  if not player_extra then
    return nil
  end
  local by_player = vim.tbl_map(function(player)
    return player_extra(update, player.seat, ctx) or false
  end, update.players)
  if not vim.iter(by_player):any(function(segments)
    return segments ~= false
  end) then
    return nil
  end
  return compose.col(vim.tbl_map(function(segments)
    return compose.row(segments or { compose.text("") })
  end, by_player))
end

---@param update games.GameUpdate
---@param me games.UserID
---@param elapsed_ms integer
---@param extras? games.Node drawn after the clocks, a line per player
---@return games.Rendered
---@return games.ClockColumn|nil
function M.render(update, me, elapsed_ms, extras)
  ---@param cell fun(player: games.PlayerSeat, i: integer): games.Node
  ---@return games.Node
  local function column(cell)
    return compose.col(vim
      .iter(ipairs(update.players))
      :map(function(i, player)
        return cell(player, i)
      end)
      :totable())
  end
  local columns = {
    column(function(player, i)
      return compose.text(player.user, theme.seat_hl(i))
    end),
    compose.pad(
      HEADER_COLUMN_GAP,
      column(function(player)
        return compose.text(player.seat, "GamesMuted")
      end)
    ),
  }
  local clocks
  if update.clock then
    local cells = M.clock_cells(update, elapsed_ms)
    clocks = {
      cols = vim.tbl_map(function(line)
        return #line + HEADER_COLUMN_GAP
      end, compose.render(compose.row(columns)).lines),
      width = M.widest_clock(cells),
    }
    columns[#columns + 1] = compose.pad(
      HEADER_COLUMN_GAP,
      column(function(_, i)
        return compose.text(cells[i].text, cells[i].hl)
      end)
    )
  end
  if extras then
    columns[#columns + 1] = compose.pad(HEADER_COLUMN_GAP, extras)
  end
  columns[#columns + 1] = compose.pad(
    HEADER_COLUMN_GAP,
    column(function(player)
      return turn_marker(update, player.user, me)
    end)
  )
  local header = compose.render(
    compose.col({ compose.row(columns), status_banner(update, me) or compose.text("") })
  )
  return header, clocks
end

---Rewrites one clock, which keeps its byte length, and its low-time highlight.
---@param buf integer
---@param namespace integer the extmark namespace the board paints in
---@param row integer
---@param col integer
---@param width integer
---@param cell games.ClockCell
function M.paint_clock(buf, namespace, row, col, width, cell)
  local stale = vim.api.nvim_buf_get_extmarks(
    buf,
    namespace,
    { row, col },
    { row, col + width - 1 },
    {}
  )
  for _, mark in ipairs(stale) do
    vim.api.nvim_buf_del_extmark(buf, namespace, mark[1])
  end
  local padded = cell.text .. text_util.spaces(width - #cell.text)
  paint.with_modifiable(buf, function()
    vim.api.nvim_buf_set_text(buf, row, col, row, col + width, { padded })
  end)
  if cell.hl then
    vim.api.nvim_buf_set_extmark(
      buf,
      namespace,
      row,
      col,
      { end_col = col + #cell.text, hl_group = cell.hl }
    )
  end
end

return M
