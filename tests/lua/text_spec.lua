local helpers = require("helpers")
local text = require("games.ui.text")

local assert_eq = helpers.assert_eq

local T = {}

function T.width_counts_display_columns_as_strdisplaywidth_does_tabs_included()
  for _, s in ipairs({ "a♔b", "和和", "a\tb", "\t" }) do
    assert_eq(vim.fn.strdisplaywidth(s), text.width(s))
  end
end

function T.fit_to_width_cuts_then_pads_and_widest_measures_by_text_of()
  assert_eq("alexa", text.fit_to_width("alexandra", 5))
  assert_eq("bo   ", text.fit_to_width("bo", 5))
  assert_eq(
    4,
    text.widest({ { name = "ann" }, { name = "bert" } }, function(player)
      return player.name
    end)
  )
  assert_eq({ "a", "♔", "b" }, text.chars("a♔b"))
end

return T
