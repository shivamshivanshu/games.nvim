local helpers = require("helpers")
local predicates = require("games.core.predicates")

local assert_eq = helpers.assert_eq

local T = {}

function T.control_characters_are_c0_del_and_c1_but_not_other_text()
  local cases = {
    { text = "r1c1=5", control = false },
    { text = "é ♔", control = false },
    { text = "a\0b", control = true },
    { text = "a\tb", control = true },
    { text = "a\127", control = true },
    { text = "\194\128", control = true },
    { text = "a\194\133b", control = true },
    { text = "\194\159", control = true },
    { text = "\194\160", control = false },
  }
  for _, case in ipairs(cases) do
    assert_eq(
      { text = case.text, control = case.control },
      { text = case.text, control = predicates.has_control_character(case.text) }
    )
  end
end

return T
