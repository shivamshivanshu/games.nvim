local helpers = require("helpers")
local pgn = require("games.games.chess.pgn")

local assert_eq = helpers.assert_eq

local NOON_UTC_2026_10_21 = 1792584000000

---@param history_san string[]
---@param result games.WireResult|nil
---@return games.ChessUpdate
local function chess_game(history_san, result)
  return helpers.chess_update({
    seq = #history_san,
    to_act = {},
    controls = {},
    result = result,
    view = { fen = "unused", history_san = history_san, in_check = false },
    started_at = NOON_UTC_2026_10_21,
  })
end

---@param pgn_text string
---@return string
local function result_tag(pgn_text)
  return pgn_text:match('%[Result "([^"]*)"%]')
end

local T = {}

function T.fools_mate_is_exact_pgn()
  local game = chess_game(
    { "f3", "e5", "g4", "Qh4#" },
    { kind = "decisive", winners = { "bob" }, reason = "checkmate" }
  )
  assert_eq(
    table.concat({
      '[Event "games.nvim"]',
      '[Site "?"]',
      ('[Date "%s"]'):format(os.date("%Y.%m.%d", NOON_UTC_2026_10_21 / 1000)),
      '[Round "-"]',
      '[White "alice"]',
      '[Black "bob"]',
      '[Result "0-1"]',
      "",
      "1. f3 e5 2. g4 Qh4# 0-1",
      "",
    }, "\n"),
    pgn.build(game)
  )
end

function T.result_follows_the_winner_seat_or_the_kind()
  local cases = {
    { result = { kind = "decisive", winners = { "alice" }, reason = "resign" }, want = "1-0" },
    { result = { kind = "decisive", winners = { "bob" }, reason = "timeout" }, want = "0-1" },
    { result = { kind = "draw", winners = {}, reason = "stalemate" }, want = "1/2-1/2" },
    { result = { kind = "aborted", winners = {}, reason = "aborted" }, want = "*" },
    { result = nil, want = "*" },
  }
  for _, case in ipairs(cases) do
    local text = pgn.build(chess_game({ "e4" }, case.result))
    assert_eq(case.want, result_tag(text))
    assert_eq(case.want, vim.split(text, "\n", { trimempty = true })[9]:match("(%S+)$"))
  end
end

function T.long_movetext_wraps_within_the_export_width()
  local history = {}
  for index = 1, 60 do
    history[index] = index % 2 == 1 and "Nf3" or "Nf6"
  end
  local lines = vim.split(pgn.build(chess_game(history, nil)), "\n", { trimempty = true })
  local movetext = vim.list_slice(lines, 9)
  assert(#movetext > 1, vim.inspect(movetext))
  for _, line in ipairs(movetext) do
    assert(#line <= 79, line)
  end
  local moves, move_numbers, result = 60, 30, 1
  assert_eq(moves + move_numbers + result, #vim.split(table.concat(movetext, " "), " "))
end

return T
