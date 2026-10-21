local cursor = require("games.ui.cursor")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

---@type games.CursorBounds
local BOUNDS = { rows = 3, cols = 4 }

---@param view games.ViewOptions
---@param keys string
---@return games.GridCursor the cursor after pressing each of keys in turn
local function after(view, keys)
  local by_key = {}
  for _, action in ipairs(vim.list_extend(cursor.moves(BOUNDS), { cursor.rotate() })) do
    by_key[action.key] = action.local_fn
  end
  for key in keys:gmatch(".") do
    view = by_key[key](view)
  end
  return cursor.of(view)
end

local T = {}

function T.the_cursor_starts_top_left_lying_across_and_moves_within_its_bounds()
  local view = { glyphs = "ascii", flipped = false }
  for _, case in ipairs({
    { keys = "", want = { row = 1, col = 1, orientation = "across" } },
    { keys = "jl", want = { row = 2, col = 2, orientation = "across" } },
    { keys = "hk", want = { row = 1, col = 1, orientation = "across" } },
    { keys = "jjjjllllll", want = { row = 3, col = 4, orientation = "across" } },
    { keys = "jrl", want = { row = 2, col = 2, orientation = "down" } },
    { keys = "rr", want = { row = 1, col = 1, orientation = "across" } },
  }) do
    assert_eq({ case.keys, case.want }, { case.keys, after(view, case.keys) })
  end
  assert_eq(nil, view.cursor)
end

function T.moving_keys_share_one_label_and_follow_their_availability()
  local available = function()
    return false
  end
  local moves = cursor.moves(BOUNDS, available)
  assert_eq(
    { "h", "j", "k", "l" },
    vim.tbl_map(function(m)
      return m.key
    end, moves)
  )
  assert(vim.iter(moves):all(function(m)
    return m.label == "move" and m.available == available
  end))
  assert_eq({ key = "r", label = "rotate", available = available }, {
    key = cursor.rotate(available).key,
    label = cursor.rotate(available).label,
    available = cursor.rotate(available).available,
  })
end

function T.a_footprint_runs_right_across_or_down_and_stops_at_the_edge()
  assert_eq(
    { { row = 2, col = 2 }, { row = 2, col = 3 }, { row = 2, col = 4 } },
    cursor.footprint({ row = 2, col = 2, orientation = "across" }, 3, BOUNDS)
  )
  assert_eq(
    { { row = 2, col = 2 }, { row = 3, col = 2 } },
    cursor.footprint({ row = 2, col = 2, orientation = "down" }, 3, BOUNDS)
  )
end

return T
