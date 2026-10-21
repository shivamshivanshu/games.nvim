local Client = require("games.core.client")
local Conn = require("games.core.conn")
local controls = require("games.core.controls")
local describe = require("games.core.describe")
local helpers = require("helpers")
local types = require("games.types")

local assert_eq = helpers.assert_eq

local T = {}

function T.a_control_sent_while_disconnected_fails_as_an_error_naming_the_game()
  local client = Client.new(Conn.new({ host = "127.0.0.1", port = 1 }))
  local update = helpers.chess_update()
  local notified = {}
  helpers.with_overrides({
    {
      vim,
      "notify",
      function(message, level)
        table.insert(notified, { message, level })
      end,
    },
  }, function()
    controls.send(
      client,
      update,
      types.Control.RESIGN,
      { label = "resign" },
      controls.report_failure(update, "alice")
    )
    helpers.wait_for(function()
      return #notified > 0
    end, "the failure")
  end)
  assert_eq(
    { { "games: " .. describe.title(update, "alice") .. ": not connected", vim.log.levels.ERROR } },
    notified
  )
end

return T
