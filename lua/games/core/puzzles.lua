local formats = require("games.core.puzzle_formats")
local fs = require("games.core.fs")
local json = require("games.core.json")
local picker = require("games.ui.picker")

---A puzzle set as stored in its file.
---@class games.PuzzleSet
---@field entries games.PuzzleEntry[]
---@field next integer 1-based: the entry :Games puzzle starts next

---@class games.PuzzleImport
---@field set string
---@field imported integer entries written to the set
---@field failures games.PuzzleFailure[]

local SET_NAME_PATTERN = "^[%w_-]+$"
local SET_SUFFIX = ".json"
---Names the most recently imported set; it has no SET_SUFFIX, so no set can be called this.
local LATEST_FILE = "latest"
---Failures an import summary lists one by one before it only counts the rest.
local LISTED_FAILURES = 10
---@type games.PuzzleSide[]
local SIDES = { "white", "black" }
---@type table<string, fun(text: string): games.PuzzleParse> by lowercase file extension
local PARSERS = { csv = formats.parse_csv, pgn = formats.parse_pgn }

local M = {}

---Where the sets live; a function, so tests can point it elsewhere.
---@return string
function M.dir()
  return vim.fs.joinpath(vim.fn.stdpath("data"), "games", "puzzles")
end

---@param name string
---@return string
local function set_path(name)
  return vim.fs.joinpath(M.dir(), name .. SET_SUFFIX)
end

---@param name any
---@return boolean
function M.is_set_name(name)
  return type(name) == "string" and name:match(SET_NAME_PATTERN) ~= nil
end

---The saved sets, sorted.
---@return string[]
function M.set_names()
  local paths = vim.fn.glob(vim.fs.joinpath(M.dir(), "*" .. SET_SUFFIX), false, true)
  return vim.tbl_map(function(path)
    return vim.fn.fnamemodify(path, ":t:r")
  end, paths)
end

---The chess_puzzle options that start entry.
---@param entry games.PuzzleEntry
---@return games.Options
function M.options(entry)
  return { mode = entry.mode, fen = entry.fen, moves = entry.moves, player = entry.player }
end

---@param task fun(): any runs inside fs.run
---@param on_done fun(err: string|nil, result: any)
local function run(task, on_done)
  fs.run(task, function(ok, result)
    if ok then
      on_done(nil, result)
    else
      on_done(tostring(result))
    end
  end)
end

---@param name string
---@param set games.PuzzleSet
local function write_set(name, set)
  fs.mkdir_p(M.dir())
  fs.write_atomic(set_path(name), vim.json.encode(set))
end

---Asks once which side to play in every entry that has none; cancelling answers nothing.
---@param entries games.PuzzleEntry[]
---@param on_entries fun(entries: games.PuzzleEntry[])
local function choose_study_side(entries, on_entries)
  local unsided = #vim.tbl_filter(function(entry)
    return entry.player == nil
  end, entries)
  if unsided == 0 then
    on_entries(entries)
    return
  end
  picker.pick(SIDES, { prompt = ("Side to study in %d games:"):format(unsided) }, function(side)
    on_entries(vim.tbl_map(function(entry)
      return vim.tbl_extend("keep", entry, { player = side })
    end, entries))
  end)
end

---Imports the CSV or PGN file at path into the set name, by default the file's basename, replacing
---any set of that name and making it the latest. Entries that cannot be read are reported in the
---result and the rest are imported; a file with none leaves the sets as they are.
---@param path string
---@param name string|nil
---@param on_done fun(err: string|nil, result: games.PuzzleImport|nil)
function M.import(path, name, on_done)
  local parse = PARSERS[vim.fn.fnamemodify(path, ":e"):lower()]
  local set_name = name or vim.fn.fnamemodify(path, ":t:r")
  if not parse then
    on_done(("%s is not a .csv or .pgn file"):format(path))
    return
  end
  if not M.is_set_name(set_name) then
    on_done(("set name %q may hold only letters, digits, _ and -"):format(set_name))
    return
  end
  run(function()
    return fs.read(vim.fn.fnamemodify(path, ":p"))
  end, function(read_err, text)
    if read_err or not text then
      on_done(read_err or ("no file %s"):format(path))
      return
    end
    local parsed = parse(text)
    ---@type games.PuzzleImport
    local result = { set = set_name, imported = #parsed.entries, failures = parsed.failures }
    if #parsed.entries == 0 then
      on_done(nil, result)
      return
    end
    choose_study_side(parsed.entries, function(entries)
      run(function()
        write_set(set_name, { entries = entries, next = 1 })
        fs.write_atomic(vim.fs.joinpath(M.dir(), LATEST_FILE), set_name)
      end, function(write_err)
        on_done(write_err, not write_err and result or nil)
      end)
    end)
  end)
end

---Loads the set name, or the latest imported one when name is nil.
---@param name string|nil
---@param on_done fun(err: string|nil, name: string|nil, set: games.PuzzleSet|nil)
function M.load(name, on_done)
  if name and not M.is_set_name(name) then
    on_done(("no puzzle set %q"):format(name))
    return
  end
  run(function()
    local set_name = name or vim.trim(fs.read(vim.fs.joinpath(M.dir(), LATEST_FILE)) or "")
    if set_name == "" then
      error("no puzzle sets: :Games import a .csv or .pgn file first", 0)
    end
    local text = fs.read(set_path(set_name))
    if not text then
      error(("no puzzle set %q"):format(set_name), 0)
    end
    return { name = set_name, set = json.decode(text) }
  end, function(err, loaded)
    if err then
      on_done(err)
      return
    end
    on_done(nil, loaded.name, loaded.set)
  end)
end

---Moves the set name past the entry it starts next.
---@param name string
---@param set games.PuzzleSet as loaded
---@param on_done fun(err: string|nil)
function M.advance(name, set, on_done)
  run(function()
    write_set(name, { entries = set.entries, next = set.next + 1 })
  end, on_done)
end

---The notice for an import: the count imported, then each failure, up to LISTED_FAILURES of them.
---@param result games.PuzzleImport
---@return string
function M.import_summary(result)
  local failed = #result.failures
  local imported = result.imported > 0
      and ("games: imported %d into puzzle set %s"):format(result.imported, result.set)
    or "games: imported nothing"
  local lines = { imported .. (failed > 0 and (", %d failed:"):format(failed) or "") }
  for i = 1, math.min(failed, LISTED_FAILURES) do
    local failure = result.failures[i]
    lines[#lines + 1] = ("  %s: %s"):format(failure.where, failure.reason)
  end
  if failed > LISTED_FAILURES then
    lines[#lines + 1] = ("  and %d more"):format(failed - LISTED_FAILURES)
  end
  return table.concat(lines, "\n")
end

return M
