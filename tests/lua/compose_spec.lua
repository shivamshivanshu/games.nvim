local compose = require("games.ui.compose")
local helpers = require("helpers")
local text_util = require("games.ui.text")

local assert_eq = helpers.assert_eq

---Candidates of sizes n x n down to 1 x 1, counting how many get built.
---@param n integer
---@return (fun(): games.Node)[]
---@return {count: integer}
local function square_candidates(n)
  local builds = { count = 0 }
  local candidates = {}
  for size = n, 1, -1 do
    candidates[#candidates + 1] = function()
      builds.count = builds.count + 1
      return compose.box({ w = size, h = size, child = compose.text(tostring(size)) })
    end
  end
  return candidates, builds
end

local T = {}

function T.box_centres_child_with_odd_padding_right_and_bottom()
  local child = compose.col({ compose.text("ab", "GamesKey"), compose.text("c") })
  local rendered = compose.render(compose.box({ w = 5, h = 5, hl = "GamesWin", child = child }))

  assert_eq({ "     ", " ab  ", " c   ", "     ", "     " }, rendered.lines)
  local box_marks = vim.tbl_filter(function(mark)
    return mark.hl == "GamesWin"
  end, rendered.extmarks)
  assert_eq(5, #box_marks)
  assert_eq({ row = 4, col = 0, end_col = 5, hl = "GamesWin" }, box_marks[5])
  assert_eq({ row = 1, col = 1, end_col = 3, hl = "GamesKey" }, rendered.extmarks[6])
end

function T.measure_reports_display_size()
  local node =
    compose.row({ compose.text("♟♟"), compose.col({ compose.text("x"), compose.text("") }) })
  assert_eq({ w = 3, h = 2 }, compose.measure(node))
end

function T.measure_matches_the_rendered_size_of_every_node_kind()
  local nodes = {
    compose.text("♟ab"),
    compose.pad(3, compose.col({ compose.text("x"), compose.text("long") })),
    compose.box({ w = 6, h = 4, child = compose.text("ab") }),
    compose.row({
      compose.box({ w = 2, h = 3, child = compose.text("k") }),
      compose.col({ compose.text("wide one"), compose.pad(1, compose.text("y")) }),
      compose.text(""),
    }),
    compose.col({}),
  }
  for _, node in ipairs(nodes) do
    local lines = compose.render(node).lines
    assert_eq({ w = text_util.max_display_width(lines), h = #lines }, compose.measure(node))
  end
end

function T.first_fit_takes_the_largest_fitting_candidate_probing_few()
  local candidates, builds = square_candidates(64)
  local node = compose.first_fit({ w = 40, h = 23 }, candidates)

  assert_eq({ w = 23, h = 23 }, compose.measure(assert(node)))
  assert(builds.count <= 7, builds.count)
end

function T.first_fit_is_nil_when_nothing_fits()
  local candidates = square_candidates(3)
  assert_eq(nil, compose.first_fit({ w = 0, h = 5 }, candidates))
end

return T
