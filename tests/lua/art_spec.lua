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

return {
  pick_takes_the_largest_entry_that_fits = function()
    assert_eq(BLOCK4, art.pick(SET, "piece", { w = 8, h = 4 }))
    assert_eq(BLOCK3, art.pick(SET, "piece", { w = 3, h = 9 }))
    assert_eq(DOT, art.pick(SET, "piece", { w = 1, h = 2 }))
    assert_eq(DOT, art.pick(SET, "piece", { w = 0, h = 0 }))
  end,

  ambiwidth_double_draws_no_multi_line_art_and_skips_widened_glyphs = function()
    with_ambiwidth("double", function()
      assert_eq(PERIOD, art.pick(SET, "piece", { w = 8, h = 4 }))
      assert_eq(".", art.glyph(SET, "piece"))
    end)
    with_ambiwidth("single", function()
      assert_eq(BLOCK4, art.pick(SET, "piece", { w = 8, h = 4 }))
      assert_eq("·", art.glyph(SET, "piece"))
    end)
  end,
}
