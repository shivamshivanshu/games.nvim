local helpers = require("helpers")
local paint = require("games.ui.paint")
local pacman = require("games.games.pacman")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local NAMESPACE = vim.api.nvim_create_namespace("games.paint_spec")
local CATALOGUED = helpers.catalogued("pacman")

---@param rows string[]
---@param extmarks? games.Extmark[]
---@return games.Rendered
local function frame(rows, extmarks)
  return { lines = rows, marks = helpers.marks_of(extmarks or {}) }
end

---@param row integer
---@param col integer
---@param end_col integer
---@param hl games.Hl
---@return games.Extmark
local function mark(row, col, end_col, hl)
  return { row = row, col = col, end_col = end_col, hl = hl }
end

---@param view_overrides table
---@param text_area games.Size
---@return games.Rendered
local function pacman_frame(view_overrides, text_area)
  local update = helpers.game_update("pacman", {
    seq = 40,
    players = { { user = "alice", seat = "player" } },
    to_act = { "alice" },
    controls = { "resign" },
    view = {
      maze = { "#######", "#.o...#", "#.##-##", "#.....#", "#######" },
      pacman = { row = 4, col = 3, facing = "left" },
      ghosts = {
        { name = "blinky", row = 2, col = 5, facing = "left", mode = "chase" },
        { name = "pinky", row = 4, col = 6, facing = "up", mode = "frightened" },
      },
      score = 1230,
      lives = 2,
      level = 2,
      levels = 3,
      food_left = 7,
      fright_ticks = 12,
      paused = false,
      can_turn = true,
      can_pause = true,
    },
  }, { view = view_overrides })
  local view = helpers.view_options("pacman", pacman, "unicode")
  local display = { text_area = text_area, ambiwidth = "single", seats = 1 }
  return (shell.render(update, pacman, view, "alice", 0, display, CATALOGUED))
end

---The lines of buf and, row by row in the order they were set, its marks in ns.
---@param buf integer
---@return {lines: string[], marks: table<integer, table[]>}
local function shown(buf)
  local by_row = {}
  local extmarks = vim.api.nvim_buf_get_extmarks(buf, NAMESPACE, 0, -1, { details = true })
  table.sort(extmarks, function(a, b)
    return a[1] < b[1]
  end)
  for _, extmark in ipairs(extmarks) do
    local _, row, col, details = unpack(extmark)
    by_row[row] = by_row[row] or {}
    table.insert(by_row[row], { col = col, details = details })
  end
  return { lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false), marks = by_row }
end

---Paints frames one after another into one buffer with paint.update and each afresh into
---another, and expects both buffers to show the same after every frame. A function among the
---frames is run on the updated buffer, as some other writer would, before the next frame.
---@param frames (games.Rendered|fun(buf: integer))[]
local function assert_updates_match_full_repaints(frames)
  local updated, repainted =
    vim.api.nvim_create_buf(false, true), vim.api.nvim_create_buf(false, true)
  local painted
  for i, rendered in ipairs(frames) do
    if type(rendered) == "function" then
      paint.with_modifiable(updated, function()
        rendered(updated)
      end)
    else
      painted = paint.update(updated, NAMESPACE, painted, rendered)
      paint(repainted, NAMESPACE, 0, -1, rendered)
      assert_eq(rendered.lines, vim.api.nvim_buf_get_lines(repainted, 0, -1, false))
      assert_eq({ i, shown(repainted) }, { i, shown(updated) })
    end
  end
  vim.api.nvim_buf_delete(updated, { force = true })
  vim.api.nvim_buf_delete(repainted, { force = true })
end

---@param first string the frame's first line, which alone differs between the frames
---@return games.Rendered
local function three_rows(first)
  return frame(
    { first, "ghijkl", "mnop" },
    { mark(0, 0, 1, "GamesKey"), mark(1, 2, 4, "GamesWin"), mark(2, 0, 4, "GamesMuted") }
  )
end

local T = {}

function T.updates_match_full_repaints_as_lines_shrink_and_highlights_move()
  assert_updates_match_full_repaints({
    frame({ "abcdef", "ghijkl", "mnop" }, { mark(0, 0, 3, "GamesKey"), mark(1, 2, 4, "GamesWin") }),
    frame({ "abc", "ghijkl", "mnop" }, { mark(0, 0, 3, "GamesKey"), mark(1, 2, 4, "GamesWin") }),
    frame({ "abc", "ghijkl", "mnop" }, { mark(0, 0, 3, "GamesKey"), mark(1, 3, 5, "GamesWin") }),
    frame({ "abc", "ghijkl", "mnop" }, {
      mark(0, 0, 3, "GamesKey"),
      mark(1, 3, 5, { "GamesWin", "GamesKey" }),
      mark(2, 0, 4, "GamesMuted"),
    }),
    frame({ "abc", "ghijkl", "mnop" }, {
      mark(1, 0, 6, "GamesMuted"),
      mark(1, 3, 5, { "GamesWin", "GamesMuted" }),
    }),
    frame({ "ab", "gh" }, { mark(1, 0, 2, "GamesKey") }),
    frame({ "ab", "gh", "", "footer" }, { mark(3, 0, 6, "GamesKey") }),
  })
end

function T.updates_restore_lines_another_writer_changed_though_the_frame_did_not()
  assert_updates_match_full_repaints({
    three_rows("a"),
    function(buf)
      vim.api.nvim_buf_set_lines(buf, 1, 2, true, { "edited" })
    end,
    three_rows("b"),
    function(buf)
      vim.api.nvim_buf_set_lines(buf, -1, -1, true, { "appended" })
    end,
    three_rows("c"),
    function(buf)
      vim.api.nvim_buf_set_lines(buf, 0, -1, true, vim.api.nvim_buf_get_lines(buf, 0, -1, true))
    end,
    three_rows("d"),
  })
end

function T.an_update_after_one_that_raised_partway_repaints_what_it_left()
  local buf = vim.api.nvim_create_buf(false, true)
  local painted = paint.update(buf, NAMESPACE, nil, three_rows("a"))
  local out_of_range = frame({ "z", "ghijkl", "mnop" }, { mark(1, 40, 41, "GamesWin") })
  assert(not pcall(paint.update, buf, NAMESPACE, painted, out_of_range))
  paint.update(buf, NAMESPACE, painted, three_rows("a"))

  local repainted = vim.api.nvim_create_buf(false, true)
  paint(repainted, NAMESPACE, 0, -1, three_rows("a"))
  assert_eq(shown(repainted), shown(buf))
  vim.api.nvim_buf_delete(buf, { force = true })
  vim.api.nvim_buf_delete(repainted, { force = true })
end

function T.updates_match_full_repaints_across_pacman_moves_a_shrinking_board_and_a_wrapping_footer()
  local wide, medium, narrow = { w = 120, h = 40 }, { w = 40, h = 40 }, { w = 24, h = 40 }
  local moved = {
    pacman = { row = 4, col = 2, facing = "left" },
    ghosts = { { name = "blinky", row = 2, col = 4, facing = "left", mode = "chase" } },
  }
  local frames = {
    pacman_frame({}, wide),
    pacman_frame(moved, wide),
    pacman_frame(vim.tbl_extend("force", moved, { paused = true, score = 7 }), wide),
    pacman_frame(moved, medium),
    pacman_frame(moved, narrow),
    pacman_frame({}, narrow),
    pacman_frame({}, wide),
  }
  assert(#frames[3].lines == #frames[2].lines, "a move keeps the line count")
  assert(#frames[4].lines < #frames[3].lines, "the medium frame shrinks the board")
  assert(#frames[5].lines > #frames[4].lines, "the narrow frame wraps its footer onto more rows")
  assert_updates_match_full_repaints(frames)
end

return T
