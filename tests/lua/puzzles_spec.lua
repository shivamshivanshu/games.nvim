local formats = require("games.core.puzzle_formats")
local helpers = require("helpers")
local json = require("games.core.json")
local puzzles = require("games.core.puzzles")

local assert_eq = helpers.assert_eq

local FIXTURES = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h")
  .. "/fixtures/puzzles"
local CSV = FIXTURES .. "/lichess.csv"
local PGN = FIXTURES .. "/games.pgn"
local SCHOLARS_FEN = "r1bqkbnr/pppp1ppp/2n5/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 3 3"

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
---@return string|nil err
---@return games.PuzzleImport|nil
local function import(path, name)
  local done, err, result = false, nil, nil
  puzzles.import(path, name, function(import_err, import_result)
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
  assert_eq({ entries = CSV_ENTRIES, failures = CSV_FAILURES }, formats.parse_csv(read(CSV)))
end

function T.a_csv_without_the_lichess_columns_imports_nothing()
  assert_eq({
    entries = {},
    failures = { { where = "line 1", reason = "the header needs PuzzleId, FEN and Moves" } },
  }, formats.parse_csv("Id,Position,Line\r\n1,8/8/8/8/8/8/8/8 w - - 0 1,e4\r\n"))
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
      { id = "game 3", mode = "puzzle", fen = SCHOLARS_FEN, moves = "Nf6 Qxf7#", player = "black" },
    },
    failures = {
      { where = "game 4", reason = "an unbalanced comment or variation" },
      { where = "game 5", reason = "no moves" },
    },
  }, formats.parse_pgn(read(PGN)))
end

function T.importing_a_csv_writes_the_set_from_its_first_entry_and_makes_it_the_latest()
  with_sets_dir(function(dir)
    local err, result = import(CSV, nil)
    assert_eq(nil, err)
    assert_eq({ set = "lichess", imported = 4, failures = CSV_FAILURES }, result)
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

return T
