local formats = require("games.core.puzzle_formats")
local fs = require("games.core.fs")
local helpers = require("helpers")
local json = require("games.core.json")
local puzzles = require("games.core.puzzles")

local assert_eq = helpers.assert_eq

local FIXTURES = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h")
  .. "/fixtures/puzzles"
local CSV = FIXTURES .. "/lichess.csv"
local PGN = FIXTURES .. "/games.pgn"
local SCHOLARS_FEN = "r1bqkbnr/pppp1ppp/2n5/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 3 3"
local MAX_ENTRIES = 100

---@type games.PuzzleEntry[]
local CSV_ENTRIES = {
  {
    id = "00008",
    mode = "puzzle",
    fen = "r6k/pp2r2p/4Rp1Q/3p4/8/1N1P2R1/PqP2bPP/7K b - - 0 24",
    moves = "f2g3 e6e7 b2b1 b3c1 b1c1 h6c1",
    player = "white",
  },
  {
    id = "0000D",
    mode = "puzzle",
    fen = "5rk1/1p3ppp/pq3b2/8/8/1P1Q1N2/P4PPP/3R2K1 w - - 2 27",
    moves = "d3d6 f8d8 d6d8 f6d8",
    player = "black",
  },
  {
    id = "0009B",
    mode = "puzzle",
    fen = "r2qr1k1/b1p2ppp/pp4n1/P1P1p3/4P1n1/B2P2Pb/3NBP1P/RN1QR1K1 b - - 1 16",
    moves = "b6c5 e2g4 h3g4 d1g4",
    player = "white",
  },
  { id = "SCHOL", mode = "puzzle", fen = SCHOLARS_FEN, moves = "g8f6 h5f7", player = "white" },
}

---@type games.PuzzleFailure[]
local CSV_FAILURES = {
  { where = "line 7", reason = "no moves" },
  { where = "line 8", reason = "the FEN has no side to move" },
}

---@param path string
---@return string
local function read(path)
  return assert(helpers.read_file(path), path)
end

---What reader makes of lines.
---@param reader fun(sink: games.PuzzleSink): games.PuzzleReader
---@param lines fun(): string|nil
---@return {entries: games.PuzzleEntry[], failures: games.PuzzleFailure[]}
local function parse(reader, lines)
  local parsed = { entries = {}, failures = {} }
  local read = reader({
    entry = function(entry)
      table.insert(parsed.entries, entry)
    end,
    failure = function(failure)
      table.insert(parsed.failures, failure)
    end,
  })
  for line in lines do
    read.line(line)
  end
  read.finish()
  return parsed
end

---@param text string
---@return fun(): string|nil
local function lines_of(text)
  return vim.gsplit(text, "\n", { plain = true })
end

---Runs fn with the puzzle sets in a fresh temp dir.
---@param fn fun(dir: string)
local function with_sets_dir(fn)
  helpers.with_temp_dir(function(dir)
    helpers.with_overrides({
      {
        puzzles,
        "dir",
        function()
          return dir
        end,
      },
    }, function()
      fn(dir)
    end)
  end)
end

---Imports path into name, waiting for the result.
---@param path string
---@param name string|nil
---@param opts? table over the import's options
---@return string|nil err
---@return games.PuzzleImport|nil
local function import(path, name, opts)
  local done, err, result = false, nil, nil
  local import_opts =
    vim.tbl_extend("force", { name = name, max_entries = MAX_ENTRIES }, opts or {})
  puzzles.import(path, import_opts, function(import_err, import_result)
    done, err, result = true, import_err, import_result
  end)
  helpers.wait_for(function()
    return done
  end, "import of " .. path)
  return err, result
end

---@param dir string
---@param name string
---@return games.PuzzleSet
local function saved_set(dir, name)
  return json.decode(read(dir .. "/" .. name .. ".json"))
end

local T = {}

function T.a_lichess_csv_gives_one_puzzle_per_row_for_the_side_not_to_move()
  assert_eq(
    { entries = CSV_ENTRIES, failures = CSV_FAILURES },
    parse(formats.csv_reader, assert(fs.lines(CSV)))
  )
end

function T.a_csv_without_the_lichess_columns_imports_nothing()
  assert_eq({
    entries = {},
    failures = { { where = "line 1", reason = "the header needs PuzzleId, FEN and Moves" } },
  }, parse(formats.csv_reader, lines_of("Id,Position,Line\n1,8/8/8/8/8/8/8/8 w - - 0 1,e4\n")))
end

function T.a_pgn_strips_everything_but_the_moves_and_reports_bad_games()
  assert_eq({
    entries = {
      {
        id = "game 1",
        mode = "study",
        fen = "",
        moves = "e4 e5 Nf3 Nc6 Bb5 a6 Ba4 Nf6 O-O Be7",
      },
      {
        id = "game 2",
        mode = "puzzle",
        fen = "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1",
        moves = "Ra8#",
        player = "white",
      },
      {
        id = "game 3",
        mode = "puzzle",
        fen = "r5k1/8/8/8/8/8/5PPP/6K1 b - - 0 1",
        moves = "Ra1#",
        player = "black",
      },
    },
    failures = {
      { where = "game 4", reason = "no moves" },
      { where = "game 5", reason = "an unbalanced comment or variation" },
    },
  }, parse(formats.pgn_reader, assert(fs.lines(PGN))))
end

function T.importing_a_csv_writes_the_set_from_its_first_entry_and_makes_it_the_latest()
  with_sets_dir(function(dir)
    local err, result = import(CSV, nil)
    assert_eq(nil, err)
    assert_eq({ set = "lichess", imported = 4, capped = false, failures = CSV_FAILURES }, result)
    assert_eq({ entries = CSV_ENTRIES, next = 1 }, saved_set(dir, "lichess"))
    assert_eq("lichess", read(dir .. "/latest"))
    assert_eq({ "lichess" }, puzzles.set_names())
    assert_eq(
      table.concat({
        "games: imported 4 into puzzle set lichess, 2 failed:",
        "  line 7: no moves",
        "  line 8: the FEN has no side to move",
      }, "\n"),
      puzzles.import_summary(assert(result))
    )
  end)
end

function T.importing_a_pgn_asks_once_which_side_its_studies_play()
  with_sets_dir(function(dir)
    local asked = {}
    helpers.with_overrides({
      {
        vim.ui,
        "select",
        function(items, opts, on_choice)
          table.insert(asked, { prompt = opts.prompt, items = items })
          on_choice("black")
        end,
      },
    }, function()
      local err = import(PGN, "openings")
      assert_eq(nil, err)
    end)
    assert_eq({ { prompt = "Side to study in 1 games:", items = { "white", "black" } } }, asked)
    local players = vim.tbl_map(function(entry)
      return entry.player
    end, saved_set(dir, "openings").entries)
    assert_eq({ "black", "white", "black" }, players)
  end)
end

function T.an_import_that_cannot_proceed_says_why_and_leaves_the_sets()
  with_sets_dir(function(dir)
    for _, case in ipairs({
      {
        path = FIXTURES .. "/games.txt",
        name = nil,
        err = FIXTURES .. "/games.txt is not a .csv or .pgn file",
      },
      {
        path = CSV,
        name = "my set",
        err = 'set name "my set" may hold only letters, digits, _ and -',
      },
      {
        path = FIXTURES .. "/missing.csv",
        name = nil,
        err = "no file " .. FIXTURES .. "/missing.csv",
      },
    }) do
      assert_eq(case.err, (import(case.path, case.name)))
    end
    assert_eq({}, puzzles.set_names())
    assert_eq(nil, helpers.read_file(dir .. "/latest"))
  end)
end

function T.a_file_with_no_readable_entry_writes_no_set()
  with_sets_dir(function(dir)
    local empty = dir .. "/empty.pgn"
    vim.fn.mkdir(dir, "p")
    vim.fn.writefile({ '[Event "Empty"]', "", "*" }, empty)
    local err, result = import(empty, nil)
    assert_eq(nil, err)
    assert_eq(
      "games: imported nothing, 1 failed:\n  game 1: no moves",
      puzzles.import_summary(assert(result))
    )
    assert_eq({}, puzzles.set_names())
  end)
end

---@param count integer
---@return string[] a Lichess CSV of count copies of the scholar's mate puzzle
local function csv_of(count)
  local lines = { "PuzzleId,FEN,Moves" }
  for n = 1, count do
    lines[#lines + 1] = ("P%d,%s,g8f6 h5f7"):format(n, SCHOLARS_FEN)
  end
  return lines
end

function T.an_import_keeps_the_first_max_entries_and_reads_no_further()
  with_sets_dir(function(dir)
    local max, path = 5, dir .. "/big.csv"
    vim.fn.mkdir(dir, "p")
    vim.fn.writefile(csv_of(max + 10), path)
    local progress = {}
    local err, result = import(path, nil, {
      max_entries = max,
      progress_every = 4,
      on_progress = function(lines)
        table.insert(progress, lines)
      end,
    })
    assert_eq(nil, err)
    assert_eq({ set = "big", imported = max, capped = true, failures = {} }, result)
    local ids = vim.tbl_map(function(entry)
      return entry.id
    end, saved_set(dir, "big").entries)
    assert_eq({ "P1", "P2", "P3", "P4", "P5" }, ids)
    assert_eq({ 4 }, progress)
    assert_eq(
      "games: imported 5 into puzzle set big; kept 5, more entries left unread",
      puzzles.import_summary(assert(result))
    )
  end)
end

function T.cancelling_the_side_question_cancels_the_import()
  with_sets_dir(function(dir)
    helpers.with_overrides({
      {
        vim.ui,
        "select",
        function(_, _, on_choice)
          on_choice(nil)
        end,
      },
    }, function()
      assert_eq("import cancelled", (import(PGN, "openings")))
    end)
    assert_eq({}, puzzles.set_names())
    assert_eq(nil, helpers.read_file(dir .. "/latest"))
  end)
end

function T.a_latest_that_names_no_set_is_ignored_and_no_path_outside_the_sets_is_touched()
  with_sets_dir(function(dir)
    local victim = vim.fs.joinpath(dir, "victim.json")
    local sets = vim.fs.joinpath(dir, "a", "b")
    local held = vim.json.encode({ entries = CSV_ENTRIES, next = 1 })
    vim.fn.mkdir(sets, "p")
    vim.fn.writefile({ held }, victim)
    vim.fn.writefile({ "../../victim" }, sets .. "/latest")
    local warned = {}
    helpers.with_overrides({
      {
        puzzles,
        "dir",
        function()
          return sets
        end,
      },
      {
        vim,
        "notify",
        function(message, level)
          table.insert(warned, { message, level })
        end,
      },
    }, function()
      local done, err, loaded = false, nil, nil
      puzzles.load(nil, function(load_err, _, set)
        done, err, loaded = true, load_err, set
      end)
      helpers.wait_for(function()
        return done
      end, "the load")
      assert_eq(nil, loaded)
      assert_eq(
        "no latest puzzle set: :Games puzzle <set>, or :Games import a .csv or .pgn file",
        err
      )
      assert_eq(
        '"../../victim" is not a puzzle set name',
        puzzles.advance("../../victim", CSV_ENTRIES[1])
      )
    end)
    assert_eq({
      {
        ('games: ignoring %s/latest: "../../victim" is not a puzzle set name'):format(sets),
        vim.log.levels.WARN,
      },
    }, warned)
    assert_eq(held .. "\n", read(victim))
  end)
end

function T.summaries_give_each_saved_set_its_size_and_next_entry()
  with_sets_dir(function()
    assert_eq(nil, (import(CSV, nil)))
    assert_eq(nil, puzzles.advance("lichess", CSV_ENTRIES[1]))
    assert_eq(nil, puzzles.advance("lichess", CSV_ENTRIES[1]))
    local done, summaries = false, nil
    puzzles.summaries(function(listed)
      done, summaries = true, listed
    end)
    helpers.wait_for(function()
      return done
    end, "the summaries")
    assert_eq({ { name = "lichess", entries = 4, next = 2 } }, summaries)
  end)
end

return T
