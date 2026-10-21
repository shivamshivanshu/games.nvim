local notify = require("games.core.notify")
local predicates = require("games.core.predicates")
local prompt = require("games.ui.prompt")
local puzzles = require("games.core.puzzles")
local shell = require("games.ui.shell")
local types = require("games.types")

local ERROR = vim.log.levels.ERROR
---@type games.GameType
local PUZZLE_GAME = "chess_puzzle"

local M = {}

local say = notify.say

---@return integer
local function puzzle_max_entries()
  return assert(require("games").config).puzzles.max_entries
end

---Imports args[1] into the puzzle set args[2], or one named after the file, keeping at most
---args[3] entries, or the configured max_entries.
---@param args string[]
function M.import(args)
  local path, name, max = args[1], args[2], args[3]
  if not path then
    say("import takes a file: :Games import <file.csv|file.pgn> [set] [max]", ERROR)
    return
  end
  local max_entries = max == nil and puzzle_max_entries() or tonumber(max)
  if not predicates.is_positive_integer(max_entries) then
    say(("import: max %q is not a positive whole number"):format(max), ERROR)
    return
  end
  local opts = {
    name = name,
    max_entries = max_entries,
    on_progress = function(lines)
      say(("import: read %d lines of %s"):format(lines, path))
    end,
  }
  puzzles.import(vim.fn.expand(path), opts, function(err, result)
    if err then
      say("import: " .. err, ERROR)
      return
    end
    ---@cast result games.PuzzleImport
    say(puzzles.import_summary(result), #result.failures > 0 and vim.log.levels.WARN or nil)
  end)
end

---Starts the next entry of the puzzle set name, or of the latest imported one, then moves the set
---past it; an entry the server refuses is reported and skipped.
---@param client games.Client
---@param name string|nil
function M.start_puzzle(client, name)
  puzzles.load(name, function(err, set_name, set)
    if err then
      say(err, ERROR)
      return
    end
    ---@cast set_name string
    ---@cast set games.PuzzleSet
    local entry = set.entries[set.next]
    if not entry then
      say(
        ("puzzle set %s is done, all %d played; restart it from :Games find puzzles"):format(
          set_name,
          #set.entries
        )
      )
      return
    end
    local label = ("%s %d/%d (%s)"):format(set_name, set.next, #set.entries, entry.id)
    client:game_start(
      { game = PUZZLE_GAME, options = puzzles.options(entry) },
      ---@param update games.GameUpdate|nil
      function(start_err, update)
        if start_err and start_err.code ~= types.ErrorCode.BAD_REQUEST then
          say(start_err.message, ERROR)
          return
        end
        if not (start_err or update) then
          return
        end
        local save_err = puzzles.advance(set_name, entry)
        if save_err then
          say(("puzzle set %s: %s"):format(set_name, save_err), ERROR)
        end
        if start_err then
          say(("skipped %s: %s"):format(label, start_err.message), ERROR)
          return
        end
        ---@cast update games.GameUpdate
        say("puzzle " .. label)
        shell.open(client, update.game_id)
      end
    )
  end)
end

---Sends the puzzle set name back to its first entry once confirmed.
---@param name string
function M.restart_puzzles(name)
  if not prompt.confirm(("Restart puzzle set %s from its first entry?"):format(name)) then
    return
  end
  local err = puzzles.restart(name)
  if err then
    say(("puzzle set %s: %s"):format(name, err), ERROR)
    return
  end
  say(("puzzle set %s starts again from its first entry"):format(name))
end

---@type games.SubcommandRun
function M.run(client, _, args)
  M.start_puzzle(client, args[1])
end

---@param arg_lead string
---@return string[]
function M.file_choices(arg_lead)
  return vim.fn.getcompletion(arg_lead, "file")
end

return M
