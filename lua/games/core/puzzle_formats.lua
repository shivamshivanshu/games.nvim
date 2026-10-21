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

---Where a reader puts what it reads, in file order.
---@class games.PuzzleSink
---@field entry fun(entry: games.PuzzleEntry)
---@field failure fun(failure: games.PuzzleFailure) an entry that could not be read

---Reads a puzzle file line by line, so a large one is never held whole.
---@class games.PuzzleReader
---@field line fun(line: string) the file's next line, without its line break
---@field finish fun() at the end of the file

---@type table<string, games.PuzzleSide> by the FEN's side-to-move letter
local SIDE_TO_MOVE = { w = "white", b = "black" }
---@type table<string, games.PuzzleSide> by the FEN's side-to-move letter
local SIDE_NOT_TO_MOVE = { w = "black", b = "white" }
---@type table<string, string> entry field -> Lichess CSV column
local CSV_COLUMNS = { id = "PuzzleId", fen = "FEN", moves = "Moves" }
local PGN_HEADER = '^%s*%[(%w+)%s+"(.-)"%]%s*$'
local PGN_RESULTS = { ["1-0"] = true, ["0-1"] = true, ["1/2-1/2"] = true, ["*"] = true }
---Where movetext stops being moves: a comment, a variation, or the end of either.
local PGN_SPECIAL = "[{};()]"
---Starts a line the PGN standard reserves for other programs.
local PGN_ESCAPE = "%"

local M = {}

---@param fen string
---@return string|nil letter "w" or "b", nil when fen has no side to move
local function side_letter(fen)
  return fen:match("^%S+%s+([wb])%s")
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
---@param sink games.PuzzleSink
---@return games.PuzzleReader
function M.csv_reader(sink)
  local n, columns = 0, nil
  local function no_header()
    sink.failure({ where = "line 1", reason = "the header needs PuzzleId, FEN and Moves" })
  end
  return {
    line = function(line)
      n = n + 1
      if n == 1 then
        columns = csv_columns(vim.split(line, ",", { plain = true }))
        if not columns then
          no_header()
        end
        return
      end
      if not (columns and line:match("%S")) then
        return
      end
      local entry, reason = csv_entry(vim.split(line, ",", { plain = true }), columns)
      if entry then
        sink.entry(entry)
      else
        ---@cast reason string
        sink.failure({ where = "line " .. n, reason = reason })
      end
    end,
    finish = function()
      if n == 0 then
        no_header()
      end
    end,
  }
end

---@class (private) games.PgnGame
---@field headers table<string, string>
---@field movetext string[] its lines outside comments and variations
---@field in_comment boolean inside a {} comment, which may span lines
---@field variations integer () variations open
---@field unbalanced boolean a } or ) closed nothing

---@return games.PgnGame
local function new_game()
  return { headers = {}, movetext = {}, in_comment = false, variations = 0, unbalanced = false }
end

---Adds what of line lies outside comments and variations to game's movetext.
---@param game games.PgnGame
---@param line string
local function scan_movetext(game, line)
  local kept, i = {}, 1
  while i <= #line do
    if game.in_comment then
      local close = line:find("}", i, true)
      if not close then
        break
      end
      game.in_comment, i = false, close + 1
    else
      local at = line:find(PGN_SPECIAL, i)
      if game.variations == 0 then
        kept[#kept + 1] = line:sub(i, at and at - 1 or #line)
      end
      if not at then
        break
      end
      local char = line:sub(at, at)
      if char == ";" then
        break
      elseif char == "{" then
        game.in_comment = true
      elseif char == "(" then
        game.variations = game.variations + 1
      elseif game.variations > 0 and char == ")" then
        game.variations = game.variations - 1
      else
        game.unbalanced = true
      end
      i = at + 1
    end
  end
  table.insert(game.movetext, table.concat(kept))
end

---The SAN moves of game, without NAGs, move numbers, annotations or the result.
---@param game games.PgnGame
---@return string|nil moves space-separated
---@return string|nil reason there are none
local function pgn_moves(game)
  if game.unbalanced or game.in_comment or game.variations > 0 then
    return nil, "an unbalanced comment or variation"
  end
  local sans = {}
  for token in table.concat(game.movetext, " "):gsub("%$%d+", " "):gmatch("%S+") do
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
  local moves, reason = pgn_moves(game)
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
---without is a study from the standard start, its player left nil for the importer to choose. A
---tag line inside a comment, and a % escape line, are not read.
---@param sink games.PuzzleSink
---@return games.PuzzleReader
function M.pgn_reader(sink)
  local n, current = 0, nil
  local function finish_game()
    if not current then
      return
    end
    n = n + 1
    local entry, reason = pgn_entry(current, n)
    if entry then
      sink.entry(entry)
    else
      ---@cast reason string
      sink.failure({ where = "game " .. n, reason = reason })
    end
    current = nil
  end
  return {
    line = function(line)
      local in_comment = current and current.in_comment
      if not in_comment and line:sub(1, 1) == PGN_ESCAPE then
        return
      end
      local tag, value
      if not in_comment then
        tag, value = line:match(PGN_HEADER)
      end
      if tag then
        if current and #current.movetext > 0 then
          finish_game()
        end
        current = current or new_game()
        current.headers[tag] = value
      elseif in_comment or line:match("%S") then
        current = current or new_game()
        scan_movetext(current, line)
      end
    end,
    finish = finish_game,
  }
end

---@type table<string, fun(sink: games.PuzzleSink): games.PuzzleReader> by lowercase file extension
M.READERS = { csv = M.csv_reader, pgn = M.pgn_reader }

return M
