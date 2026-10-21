local formats = require("games.core.puzzle_formats")
local fs = require("games.core.fs")
local json = require("games.core.json")
local notify = require("games.core.notify")
local picker = require("games.ui.picker")

---A puzzle set as stored in its file.
---@class games.PuzzleSet
---@field entries games.PuzzleEntry[]
---@field next integer 1-based: the entry :Games puzzle starts next

---@class games.PuzzleImport
---@field set string
---@field imported integer entries written to the set
---@field capped boolean the file holds more entries than the set's max_entries; they are left unread
---@field failures games.PuzzleFailure[]

---@class games.PuzzleImportOpts
---@field name? string the set; the file's basename by default
---@field max_entries integer the set keeps the file's first this many entries
---@field on_progress? fun(lines: integer) called every progress_every lines read
---@field progress_every? integer PROGRESS_LINES by default

---What the finder shows of a saved set.
---@class games.PuzzleSetSummary
---@field name string
---@field entries integer
---@field next integer 1-based, as in its games.PuzzleSet

local SET_NAME_PATTERN = "^[%w_-]+$"
local SET_SUFFIX = ".json"
---Names the most recently imported set; it has no SET_SUFFIX, so no set can be called this.
local LATEST_FILE = "latest"
---Failures an import summary lists one by one before it only counts the rest.
local LISTED_FAILURES = 10
local PROGRESS_LINES = 100000
---@type games.PuzzleSide[]
local SIDES = { "white", "black" }

local M = {}

---Where the sets live; a function, so tests can point it elsewhere.
---@return string
function M.dir()
  return vim.fs.joinpath(vim.fn.stdpath("data"), "games", "puzzles")
end

---@param name any
---@return boolean
function M.is_set_name(name)
  return type(name) == "string" and name:match(SET_NAME_PATTERN) ~= nil
end

---Every set file is reached through here, so a name from a file can never leave the sets' dir.
---@param name string
---@return string
local function set_path(name)
  if not M.is_set_name(name) then
    error(("%q is not a puzzle set name"):format(name), 0)
  end
  return vim.fs.joinpath(M.dir(), name .. SET_SUFFIX)
end

---@return string
local function latest_path()
  return vim.fs.joinpath(M.dir(), LATEST_FILE)
end

---The saved sets, sorted.
---@return string[]
function M.set_names()
  local paths = vim.fn.glob(vim.fs.joinpath(M.dir(), "*" .. SET_SUFFIX), false, true)
  return vim.tbl_filter(
    M.is_set_name,
    vim.tbl_map(function(path)
      return vim.fn.fnamemodify(path, ":t:r")
    end, paths)
  )
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

---@param name string
---@return games.PuzzleSet
local function read_set(name)
  local text = fs.read(set_path(name))
  if not text then
    error(("no puzzle set %q"):format(name), 0)
  end
  return json.decode(text)
end

---Asks once which side to play in every entry that has none; on_entries gets nil when the
---question is cancelled.
---@param entries games.PuzzleEntry[]
---@param on_entries fun(entries: games.PuzzleEntry[]|nil)
local function choose_study_side(entries, on_entries)
  local unsided = #vim.tbl_filter(function(entry)
    return entry.player == nil
  end, entries)
  if unsided == 0 then
    on_entries(entries)
    return
  end
  local prompt = ("Side to study in %d games:"):format(unsided)
  picker.pick(SIDES, { prompt = prompt }, function(side)
    on_entries(vim.tbl_map(function(entry)
      return vim.tbl_extend("keep", entry, { player = side })
    end, entries))
  end, function()
    on_entries(nil)
  end)
end

---@class (private) games.PuzzleFileRead
---@field entries games.PuzzleEntry[]
---@field capped boolean
---@field failures games.PuzzleFailure[]

---Reads the file at path line by line through read, keeping its first opts.max_entries entries;
---reading stops at the entry after them, so a large file is never read past the cap.
---@param path string absolute
---@param read fun(sink: games.PuzzleSink): games.PuzzleReader
---@param opts games.PuzzleImportOpts
---@return games.PuzzleFileRead
local function read_file(path, read, opts)
  local lines, close = fs.lines(path)
  if not lines then
    error(("no file %s"):format(path), 0)
  end
  ---@type games.PuzzleFileRead
  local file = { entries = {}, capped = false, failures = {} }
  local reader = read({
    entry = function(entry)
      if #file.entries < opts.max_entries then
        table.insert(file.entries, entry)
      else
        file.capped = true
      end
    end,
    failure = function(failure)
      table.insert(file.failures, failure)
    end,
  })
  local every, count = opts.progress_every or PROGRESS_LINES, 0
  local ok, err = pcall(function()
    for line in lines do
      reader.line(line)
      if file.capped then
        return
      end
      count = count + 1
      if opts.on_progress and count % every == 0 then
        opts.on_progress(count)
      end
    end
    reader.finish()
  end)
  close()
  if not ok then
    error(err, 0)
  end
  return file
end

---Imports the CSV or PGN file at path into a set, replacing any set of that name and making it
---the latest. Entries that cannot be read are reported in the result and the rest are imported;
---a file with none, or a cancelled side question, leaves the sets as they are.
---@param path string
---@param opts games.PuzzleImportOpts
---@param on_done fun(err: string|nil, result: games.PuzzleImport|nil)
function M.import(path, opts, on_done)
  local read = formats.READERS[vim.fn.fnamemodify(path, ":e"):lower()]
  local set_name = opts.name or vim.fn.fnamemodify(path, ":t:r")
  if not read then
    on_done(("%s is not a .csv or .pgn file"):format(path))
    return
  end
  if not M.is_set_name(set_name) then
    on_done(("set name %q may hold only letters, digits, _ and -"):format(set_name))
    return
  end
  run(function()
    return read_file(vim.fn.fnamemodify(path, ":p"), read, opts)
  end, function(read_err, file)
    if read_err then
      on_done(read_err)
      return
    end
    ---@cast file games.PuzzleFileRead
    ---@type games.PuzzleImport
    local result =
      { set = set_name, imported = #file.entries, capped = file.capped, failures = file.failures }
    if #file.entries == 0 then
      on_done(nil, result)
      return
    end
    choose_study_side(file.entries, function(entries)
      if not entries then
        on_done("import cancelled")
        return
      end
      run(function()
        write_set(set_name, { entries = entries, next = 1 })
        fs.write_atomic(latest_path(), set_name)
      end, function(write_err)
        on_done(write_err, not write_err and result or nil)
      end)
    end)
  end)
end

---Loads the set name, or the latest imported one when name is nil. A latest that names no set is
---ignored with a warning.
---@param name string|nil
---@param on_done fun(err: string|nil, name: string|nil, set: games.PuzzleSet|nil)
function M.load(name, on_done)
  if name and not M.is_set_name(name) then
    on_done(("no puzzle set %q"):format(name))
    return
  end
  run(function()
    local set_name = name or vim.trim(fs.read(latest_path()) or "")
    if set_name ~= "" and not M.is_set_name(set_name) then
      notify.say(
        ("ignoring %s: %q is not a puzzle set name"):format(latest_path(), set_name),
        vim.log.levels.WARN
      )
      set_name = ""
    end
    if set_name == "" then
      error("no latest puzzle set: :Games puzzle <set>, or :Games import a .csv or .pgn file", 0)
    end
    return { name = set_name, set = read_set(set_name) }
  end, function(err, loaded)
    if err then
      on_done(err)
      return
    end
    on_done(nil, loaded.name, loaded.set)
  end)
end

---Runs fn, turning what it raises into an error message.
---@param fn fun()
---@return string|nil err
local function try(fn)
  local ok, err = pcall(fn)
  return not ok and tostring(err) or nil
end

---Moves the set name past started, when started is still the entry it starts next: a set imported
---again since keeps its own place. It reads and writes the set before it returns, so no other
---write to the set can land in between.
---@param name string
---@param started games.PuzzleEntry
---@return string|nil err
function M.advance(name, started)
  return try(function()
    local set = read_set(name)
    if vim.deep_equal(set.entries[set.next], started) then
      write_set(name, { entries = set.entries, next = set.next + 1 })
    end
  end)
end

---Sends the set name back to its first entry, reading and writing it before it returns.
---@param name string
---@return string|nil err
function M.restart(name)
  return try(function()
    write_set(name, { entries = read_set(name).entries, next = 1 })
  end)
end

---Each saved set's name, size and next entry, sorted by name; a set that cannot be read is left out.
---@param on_done fun(summaries: games.PuzzleSetSummary[])
function M.summaries(on_done)
  run(function()
    local summaries = {}
    for _, name in ipairs(M.set_names()) do
      local ok, set = pcall(read_set, name)
      if ok then
        table.insert(summaries, { name = name, entries = #set.entries, next = set.next })
      end
    end
    return summaries
  end, function(_, summaries)
    on_done(summaries or {})
  end)
end

---The notice for an import: the count imported, then each failure, up to LISTED_FAILURES of them.
---@param result games.PuzzleImport
---@return string
function M.import_summary(result)
  local failed = #result.failures
  local imported = result.imported > 0
      and ("imported %d into puzzle set %s"):format(result.imported, result.set)
    or "imported nothing"
  local capped = result.capped and ("; kept %d, more entries left unread"):format(result.imported)
    or ""
  local lines = { imported .. capped .. (failed > 0 and (", %d failed:"):format(failed) or "") }
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
