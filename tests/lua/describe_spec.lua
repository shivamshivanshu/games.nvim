local describe = require("games.core.describe")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local T = {}

function T.result_text_by_kind()
  assert_eq(
    "Result: bob won (checkmate)",
    describe.result_text({ kind = "decisive", winners = { "bob" }, reason = "checkmate" })
  )
  assert_eq(
    "Result: lost (timeout)",
    describe.result_text({ kind = "decisive", winners = {}, reason = "timeout" })
  )
  assert_eq(
    "Result: draw (draw agreed)",
    describe.result_text({ kind = "draw", winners = {}, reason = "draw_agreed" })
  )
  assert_eq(
    "Result: aborted",
    describe.result_text({ kind = "aborted", winners = {}, reason = "aborted" })
  )
end

return T
