local Archive = require("games.core.archive")
local notify = require("games.core.notify")

local M = {}

---@param tally games.ArchiveTally
---@return string
local function export_summary(tally)
  local Outcome = Archive.Outcome
  local parts = {
    ("%d written"):format(tally[Outcome.WRITTEN] or 0),
    ("%d already present"):format(tally[Outcome.ALREADY_PRESENT] or 0),
  }
  local failed = (tally[Outcome.REFUSED] or 0) + (tally[Outcome.FAILED] or 0)
  if failed > 0 then
    table.insert(parts, ("%d failed"):format(failed))
  end
  return "export: " .. table.concat(parts, ", ")
end

---@type games.SubcommandRun
function M.run(client)
  local archive = require("games").archive
  if not archive then
    notify.error("export needs archive.dir in setup()")
    return
  end
  client:finished_games(function(finished)
    archive:backfill(finished, function(tally)
      notify.say(export_summary(tally))
    end)
  end)
end

return M
