local art = require("games.ui.art")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local DOT = { h = 1, w = 1, lines = { "·" } }
local PERIOD = { h = 1, w = 1, lines = { "." } }
local BLOCK3 = { h = 3, w = 2, lines = { "▄▄", "██", "▀▀" } }
local BLOCK4 =
  { h = 4, w = 4, lines = { "▗▄▄▖", "▐██▌", "▐██▌", "▝▀▀▘" } }
local SET = { piece = { DOT, PERIOD, BLOCK4, BLOCK3 } }

---@param ambiwidth string
---@param fn fun()
local function with_ambiwidth(ambiwidth, fn)
  helpers.with_overrides({ { vim.o, "ambiwidth", ambiwidth } }, fn)
end

local T = {}

function T.pick_takes_the_largest_entry_that_fits()
  assert_eq(BLOCK4, art.pick(SET, "piece", { w = 8, h = 4 }, "single"))
  assert_eq(BLOCK3, art.pick(SET, "piece", { w = 3, h = 9 }, "single"))
  assert_eq(DOT, art.pick(SET, "piece", { w = 1, h = 2 }, "single"))
  assert_eq(DOT, art.pick(SET, "piece", { w = 0, h = 0 }, "single"))
end

function T.ambiwidth_double_draws_no_multi_line_art_and_skips_widened_glyphs()
  with_ambiwidth("double", function()
    assert_eq(PERIOD, art.pick(SET, "piece", { w = 8, h = 4 }, "double"))
    assert_eq(".", art.glyph(SET, "piece", "double"))
  end)
  with_ambiwidth("single", function()
    assert_eq(BLOCK4, art.pick(SET, "piece", { w = 8, h = 4 }, "single"))
    assert_eq("·", art.glyph(SET, "piece", "single"))
  end)
end

function T.picture_draws_unicode_unless_it_widens_then_its_ascii_stand_in()
  local box = { h = 2, w = 3, lines = { "┌─┐", "└─┘" } }
  local plain = { h = 2, w = 3, lines = { "+-+", "+-+" } }
  local set = { scene = { box, plain } }
  with_ambiwidth("single", function()
    assert_eq(box.lines, art.picture(set, "scene", "single"))
  end)
  with_ambiwidth("double", function()
    assert_eq(plain.lines, art.picture(set, "scene", "double"))
    local ok, err = pcall(art.picture, { scene = { box } }, "scene", "double")
    assert(not ok and err:find('no picture of key "scene" draws', 1, true), err)
  end)
end

function T.a_key_with_no_drawable_glyph_is_an_error_not_a_misdrawn_board()
  with_ambiwidth("double", function()
    local ok, err = pcall(
      art.pick,
      { piece = { DOT, BLOCK3 } },
      "piece",
      { w = 1, h = 1 },
      "double"
    )
    assert(not ok and err:find('no glyph of key "piece" draws', 1, true), err)
  end)
end

return T
