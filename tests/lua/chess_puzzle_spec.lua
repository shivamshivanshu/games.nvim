local describe = require("games.core.describe")
local helpers = require("helpers")
local puzzle = require("games.games.chess_puzzle")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = helpers.catalogued("chess_puzzle")
local LINE = "g8f6 h5f7"
local SCHOLARS = "r1bqkbnr/pppp1ppp/2n5/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 3 3"
local AFTER_NF6 = "r1bqkb1r/pppp1ppp/2n2n2/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR w KQkq - 4 4"
local FIRST_BOARD_ROW = 3
local BOARD_LINES = 9

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("chess_puzzle", puzzle, glyphs, extra)
end

---alice's puzzle after the server's setup move Nf6, white to find Qxf7#.
---@param mode games.ChessPuzzleMode
---@param player string
---@param overrides? table
---@return games.ChessPuzzleUpdate
local function update(mode, player, overrides)
  return helpers.game_update("chess_puzzle", {
    options = { mode = mode, fen = SCHOLARS, moves = LINE, player = player },
    players = { { user = "alice", seat = "solver" } },
    to_act = { "alice" },
    controls = { "resign", "abort" },
    view = {
      fen = AFTER_NF6,
      history_san = { "Nf6" },
      last_move = { from = "g8", to = "f6" },
      legal_moves = { "h5f7" },
      first_ply = 5,
      mode = mode,
      misses = 0,
      remaining = 1,
      solution = {},
    },
  }, overrides)
end

---@param state games.ChessPuzzleUpdate
---@param result games.WireResult
---@return games.ChessPuzzleUpdate
local function finished(state, result)
  local over = vim.tbl_extend("force", state, { to_act = {}, controls = {}, result = result })
  over.view =
    vim.tbl_extend("force", state.view, { legal_moves = {}, solution = { "Nf6", "Qxf7#" } })
  return over
end

local T = {}

function T.board_faces_the_solvers_side_beside_the_status_and_hides_the_line()
  for _, case in ipairs({
    {
      player = "white",
      board = {
        "8 ♜ · ♝ ♛ ♚ ♝ · ♜   ♔ K king          3. ... Nf6   mode       puzzle",
        "7 ♟ ♟ ♟ ♟ · ♟ ♟ ♟   ♕ Q queen                      you play   white ",
        "6 · · ♞ · · ♞ · ·   ♖ R rook                       moves left 1     ",
        "5 · · · · ♟ · · ♕   ♗ B bishop                     misses     0     ",
        "4 · · ♗ · ♙ · · ·   ♘ N knight                                      ",
        "3 · · · · · · · ·   ♙   pawn: e4                                    ",
        "2 ♙ ♙ ♙ ♙ · ♙ ♙ ♙       castle: O-O                                 ",
        "1 ♖ ♘ ♗ · ♔ · ♘ ♖                                                   ",
        "  a b c d e f g h                                                   ",
      },
    },
    {
      player = "black",
      board = {
        "1 ♖ ♘ · ♔ · ♗ ♘ ♖   ♔ K king          3. ... Nf6   mode       puzzle",
        "2 ♙ ♙ ♙ · ♙ ♙ ♙ ♙   ♕ Q queen                      you play   black ",
        "3 · · · · · · · ·   ♖ R rook                       moves left 1     ",
        "4 · · · ♙ · ♗ · ·   ♗ B bishop                     misses     0     ",
        "5 ♕ · · ♟ · · · ·   ♘ N knight                                      ",
        "6 · · ♞ · · ♞ · ·   ♙   pawn: e4                                    ",
        "7 ♟ ♟ ♟ · ♟ ♟ ♟ ♟       castle: O-O                                 ",
        "8 ♜ · ♝ ♚ ♛ ♝ · ♜                                                   ",
        "  h g f e d c b a                                                   ",
      },
    },
  }) do
    local rendered = shell.render(
      update("puzzle", case.player),
      puzzle,
      view_options("unicode"),
      "alice",
      0,
      nil,
      CATALOGUED
    )
    assert_eq(
      case.board,
      vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + BOARD_LINES)
    )
    assert_eq(nil, table.concat(rendered.lines, "\n"):find("h5f7", 1, true))
  end
end

function T.a_failed_puzzle_shows_its_miss_and_the_solution_numbered_from_the_fen()
  local over =
    finished(update("puzzle", "white"), { kind = "decisive", winners = {}, reason = "wrong_move" })
  over.view.misses = 1
  local rendered = shell.render(over, puzzle, view_options("ascii"), "alice", 0, nil, CATALOGUED)
  assert_eq(
    {
      "mode       puzzle",
      "you play   white ",
      "moves left 1     ",
      "misses     1     ",
      "                 ",
      "solution         ",
      "3. ...   Nf6     ",
      "4. Qxf7#         ",
    },
    vim
      .iter(vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + 8))
      :map(function(line)
        return line:sub(-17)
      end)
      :totable()
  )
end

function T.reveal_is_offered_only_in_a_running_study()
  for _, case in ipairs({
    {
      state = update("puzzle", "white"),
      footer = "R give up  A abort  m move  F flip  ]g next game  ? help  q close",
    },
    {
      state = update("study", "white"),
      footer = "R give up  A abort  m move  n reveal  F flip  ]g next game  ? help  q close",
    },
    {
      state = finished(
        update("study", "white"),
        { kind = "decisive", winners = {}, reason = "resign" }
      ),
      footer = "F flip  ]g next game  ? help  q close",
    },
  }) do
    local rendered, _, bindings =
      shell.render(case.state, puzzle, view_options("unicode"), "alice", 0, nil, CATALOGUED)
    assert_eq(case.footer, rendered.lines[#rendered.lines])
    local reveal = vim.iter(bindings):find(function(binding)
      return binding.key == "n"
    end)
    assert_eq(case.footer:find(" n reveal ") ~= nil, reveal ~= nil)
  end
end

function T.events_read_as_the_solver_and_the_side_the_server_plays()
  local state = update("study", "black", {
    events = {
      { kind = "moved", by = "solver", san = "e5" },
      { kind = "replied", by = "solver", san = "Nf3" },
      { kind = "missed", by = "solver", san = "Nc6" },
      { kind = "revealed", by = "solver", san = "Nf6" },
    },
  })
  assert_eq(
    "alice played e5; white replied Nf3; alice tried Nc6, not the move; the move was Nf6",
    describe.events(state)
  )
end

return T
