local chess = require("games.games.chess")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local cases = require("fixtures.chess_h1")

local GOLDEN_PATH = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h")
  .. "/fixtures/chess_h1.json"
local CATALOGUED = helpers.catalogued("chess")

---@param extmarks games.Extmark[]
---@return games.Extmark[] in row, then column order
local function sorted(extmarks)
  local copy = vim.deepcopy(extmarks)
  table.sort(copy, function(a, b)
    if a.row ~= b.row then
      return a.row < b.row
    end
    return a.col < b.col
  end)
  return copy
end

local function golden()
  local file = assert(io.open(GOLDEN_PATH))
  local decoded = vim.json.decode(file:read("*a"))
  file:close()
  return decoded
end

local T = {}

function T.chess_at_cell_height_one_renders_as_before_scalable_boards()
  for _, expected in ipairs(golden()) do
    local case = cases[expected.name]
    local view = helpers.view_options("chess", case.glyphs, { flipped = case.flipped })
    local rendered, header_height =
      shell.render(case.update, chess, view, case.me, case.elapsed_ms, nil, CATALOGUED)
    helpers.assert_eq({ expected.name, expected.lines }, { expected.name, rendered.lines })
    helpers.assert_eq(
      { expected.name, sorted(expected.extmarks) },
      { expected.name, sorted(helpers.extmarks(rendered)) }
    )
    helpers.assert_eq(expected.header_height, header_height)
  end
end

return T
