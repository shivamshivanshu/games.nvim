local players = require("games.core.players")
local types = require("games.types")

local ResultKind = types.ResultKind

local EVENT = "games.nvim"
local UNKNOWN = "?"
local ROUND = "-"
local WHITE = "white"
local BLACK = "black"
local EXPORT_LINE_WIDTH = 79

---@enum games.PgnResult
local PgnResult = {
  WHITE_WINS = "1-0",
  BLACK_WINS = "0-1",
  DRAW = "1/2-1/2",
  UNDECIDED = "*",
}

local M = {}

---@param update games.GameUpdate
---@return games.PgnResult
local function pgn_result(update)
  local result = update.result
  if not result then
    return PgnResult.UNDECIDED
  end
  if result.kind == ResultKind.DRAW then
    return PgnResult.DRAW
  end
  if result.kind ~= ResultKind.DECISIVE then
    return PgnResult.UNDECIDED
  end
  if vim.tbl_contains(result.winners, players.user_of_seat(update, WHITE)) then
    return PgnResult.WHITE_WINS
  end
  if vim.tbl_contains(result.winners, players.user_of_seat(update, BLACK)) then
    return PgnResult.BLACK_WINS
  end
  return PgnResult.UNDECIDED
end

---@param name string
---@param value string
---@return string
local function tag(name, value)
  return ('[%s "%s"]'):format(name, (value:gsub('[\\"]', "\\%0")))
end

---@param history_san string[]
---@param result games.PgnResult
---@return string[] move numbers, moves and the result, in order
local function movetext_tokens(history_san, result)
  local tokens = {}
  for index, san in ipairs(history_san) do
    if index % 2 == 1 then
      tokens[#tokens + 1] = ("%d."):format((index + 1) / 2)
    end
    tokens[#tokens + 1] = san
  end
  tokens[#tokens + 1] = result
  return tokens
end

---@param tokens string[]
---@param width integer
---@return string[]
local function wrap(tokens, width)
  local lines = { tokens[1] }
  for index = 2, #tokens do
    local candidate = lines[#lines] .. " " .. tokens[index]
    if #candidate > width then
      lines[#lines + 1] = tokens[index]
    else
      lines[#lines] = candidate
    end
  end
  return lines
end

---The game as PGN export format: the Seven Tag Roster, then the movetext.
---@param update games.ChessUpdate with a view
---@return string
function M.build(update)
  local result = pgn_result(update)
  local date = os.date("%Y.%m.%d", math.floor(update.started_at / 1000)) --[[@as string]]
  local lines = {
    tag("Event", EVENT),
    tag("Site", UNKNOWN),
    tag("Date", date),
    tag("Round", ROUND),
    tag("White", players.user_of_seat(update, WHITE) or UNKNOWN),
    tag("Black", players.user_of_seat(update, BLACK) or UNKNOWN),
    tag("Result", result),
    "",
  }
  vim.list_extend(lines, wrap(movetext_tokens(update.view.history_san, result), EXPORT_LINE_WIDTH))
  return table.concat(lines, "\n") .. "\n"
end

return M
