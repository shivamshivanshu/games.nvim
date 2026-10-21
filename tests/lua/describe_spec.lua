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

function T.title_names_the_opponents_or_solo()
  local players = {
    { user = "alice", seat = "white" },
    { user = "bob", seat = "black" },
    { user = "carol", seat = "red" },
  }
  for _, case in ipairs({
    { seated = 3, title = "party vs bob, carol" },
    { seated = 1, title = "party solo" },
  }) do
    local update = { game = "party", players = vim.list_slice(players, 1, case.seated) }
    assert_eq(case.title, describe.title(update --[[@as games.GameUpdate]], "alice"))
  end
end

function T.events_read_through_the_module_s_describers_and_generically_without_one()
  package.preload["games.games.describers_test"] = function()
    return {
      layout = function() end,
      describers = {
        moved = function(event, talk)
          return ("%s moved to %s"):format(talk.name(event.by), event.to)
        end,
        passed = function()
          return nil
        end,
      },
    }
  end
  local update = helpers.game_update("describers_test", {
    players = { { user = "alice", seat = "white" } },
    events = {
      { kind = "moved", by = "white", to = "e4" },
      { kind = "moved", by = "black", to = "e5" },
      { kind = "passed", by = "white" },
      { kind = "flagged", by = "white" },
    },
  })
  local text = describe.events(update)
  package.preload["games.games.describers_test"] = nil

  assert_eq("alice moved to e4; black moved to e5; alice: passed; alice: flagged", text)
end

return T
