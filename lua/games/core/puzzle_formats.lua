---@alias games.PuzzleSide "white"|"black"

---One game a puzzle set starts: the chess_puzzle options plus an id to show.
---@class games.PuzzleEntry
---@field id string
---@field mode games.ChessPuzzleMode
---@field fen string the start position; "" for the standard one
---@field moves string the whole line, both sides, space-separated SAN or UCI
---@field player? games.PuzzleSide the side the solver plays; nil for a study whose side is not chosen yet

---@class games.PuzzleFailure
---@field where string e.g. "line 4" or "game 2"
---@field reason string

---@class games.PuzzleParse
---@field entries games.PuzzleEntry[]
---@field failures games.PuzzleFailure[] the entries that could not be read, in file order

---@type table<string, games.PuzzleSide> by the FEN's side-to-move letter
local SIDE_TO_MOVE = { w = "white", b = "black" }
---@type table<string, games.PuzzleSide> by the FEN's side-to-move letter
local SIDE_NOT_TO_MOVE = { w = "black", b = "white" }
---@type table<string, string> entry field -> Lichess CSV column
local CSV_COLUMNS = { id = "PuzzleId", fen = "FEN", moves = "Moves" }
local PGN_HEADER = '^%s*%[(%w+)%s+"(.-)"%]%s*$'
local PGN_RESULTS = { ["1-0"] = true, ["0-1"] = true, ["1/2-1/2"] = true, ["*"] = true }

local M = {}

---@param fen string
---@return string|nil letter "w" or "b", nil when fen has no side to move
local function side_letter(fen)
  return fen:match("^%S+%s+([wb])%s")
end

---@param text string
---@return string[]
local function lines_of(text)
  return vim.split((text:gsub("\r\n?", "\n")), "\n", { plain = true })
end

---@param header string[]
---@return table<string, integer>|nil each entry field's column, nil when one is missing
local function csv_columns(header)
  local columns = {}
  for field, name in pairs(CSV_COLUMNS) do
    local index = vim.fn.index(header, name)
    if index < 0 then
      return nil
    end
    columns[field] = index + 1
  end
  return columns
end

---@param cells string[]
---@param columns table<string, integer>
---@return games.PuzzleEntry|nil
---@return string|nil reason the row is not a puzzle
local function csv_entry(cells, columns)
  local id, fen = cells[columns.id] or "", cells[columns.fen] or ""
  local moves = vim.trim(cells[columns.moves] or "")
  local side = side_letter(fen)
  if id == "" then
    return nil, "no PuzzleId"
  elseif not side then
    return nil, "the FEN has no side to move"
  elseif moves == "" then
    return nil, "no moves"
  end
  return { id = id, mode = "puzzle", fen = fen, moves = moves, player = SIDE_NOT_TO_MOVE[side] }
end

---Reads a Lichess puzzle CSV: each row is a puzzle whose first move is the opponent's, so the
---solver plays the side not to move in its FEN.
---@param text string
---@return games.PuzzleParse
function M.parse_csv(text)
  local lines = lines_of(text)
  local columns = csv_columns(vim.split(lines[1], ",", { plain = true }))
  if not columns then
    return {
      entries = {},
      failures = { { where = "line 1", reason = "the header needs PuzzleId, FEN and Moves" } },
    }
  end
  local parsed = { entries = {}, failures = {} }
  for n = 2, #lines do
    if lines[n]:match("%S") then
      local entry, reason = csv_entry(vim.split(lines[n], ",", { plain = true }), columns)
      if entry then
        table.insert(parsed.entries, entry)
      else
        table.insert(parsed.failures, { where = "line " .. n, reason = reason })
      end
    end
  end
  return parsed
end

---@class (private) games.PgnGame
---@field headers table<string, string>
---@field movetext string[] its lines

---@param text string
---@return games.PgnGame[]
local function pgn_games(text)
  local games, current = {}, nil
  for _, line in ipairs(lines_of(text)) do
    local tag, value = line:match(PGN_HEADER)
    if not current or (tag and #current.movetext > 0) then
      current = { headers = {}, movetext = {} }
      table.insert(games, current)
    end
    if tag then
      current.headers[tag] = value
    elseif line:match("%S") then
      table.insert(current.movetext, line)
    end
  end
  return vim.tbl_filter(function(game)
    return #game.movetext > 0 or not vim.tbl_isempty(game.headers)
  end, games)
end

---The SAN moves of movetext, without comments, variations, NAGs, move numbers, annotations or
---the result.
---@param movetext string
---@return string|nil moves space-separated
---@return string|nil reason there are none
local function pgn_moves(movetext)
  local stripped =
    movetext:gsub("%b{}", " "):gsub(";[^\n]*", " "):gsub("%b()", " "):gsub("%$%d+", " ")
  if stripped:find("[{}()]") then
    return nil, "an unbalanced comment or variation"
  end
  local sans = {}
  for token in stripped:gmatch("%S+") do
    local san = token:gsub("^%d+%.+", ""):gsub("[!?]+$", "")
    if san ~= "" and not PGN_RESULTS[san] then
      sans[#sans + 1] = san
    end
  end
  if #sans == 0 then
    return nil, "no moves"
  end
  return table.concat(sans, " ")
end

---@param game games.PgnGame
---@param n integer
---@return games.PuzzleEntry|nil
---@return string|nil reason the game is not an entry
local function pgn_entry(game, n)
  local moves, reason = pgn_moves(table.concat(game.movetext, "\n"))
  if not moves then
    return nil, reason
  end
  local id, fen = "game " .. n, game.headers.FEN
  if not fen then
    return { id = id, mode = "study", fen = "", moves = moves }
  end
  local side = side_letter(fen)
  if not side then
    return nil, "the FEN has no side to move"
  end
  return { id = id, mode = "puzzle", fen = fen, moves = moves, player = SIDE_TO_MOVE[side] }
end

---Reads a PGN file game by game. A game with a FEN header is a puzzle for the side to move; one
---without is a study from the standard start, its player left nil for the importer to choose.
---@param text string
---@return games.PuzzleParse
function M.parse_pgn(text)
  local parsed = { entries = {}, failures = {} }
  for n, game in ipairs(pgn_games(text)) do
    local entry, reason = pgn_entry(game, n)
    if entry then
      table.insert(parsed.entries, entry)
    else
      table.insert(parsed.failures, { where = "game " .. n, reason = reason })
    end
  end
  return parsed
end

return M
