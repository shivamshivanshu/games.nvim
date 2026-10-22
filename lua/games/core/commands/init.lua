local challenge = require("games.core.commands.challenge")
local dashboard = require("games.ui.dashboard")
local export = require("games.core.commands.export")
local find = require("games.core.commands.find")
local login = require("games.core.login")
local notify = require("games.core.notify")
local puzzle = require("games.core.commands.puzzle")
local puzzles = require("games.core.puzzles")
local shell = require("games.ui.shell")
local solo = require("games.core.commands.solo")
local sources = require("games.core.sources")
local stats = require("games.core.commands.stats")

---@alias games.SubcommandRun fun(client: games.Client, me: games.UserID, args: string[])

---@class (private) games.Subcommand
---@field name string
---@field usage string
---@field run? games.SubcommandRun
---@field run_offline? fun(args: string[]) in place of run, for a subcommand that does not wait for a login
---@field arg_choices? fun(arg_lead: string): string[] completions for the first argument

local M = {}

---@type games.SubcommandRun
local function next_game(client)
  shell.open_next(client)
end

---Opens the finder over the challenges the server lists, once the catalog naming their games is in.
---@type games.SubcommandRun
local function find_challenges(client, me)
  find.find(client, me, sources.SourceName.REQUESTS)
end

---@type games.SubcommandRun
local function open_dashboard(client)
  dashboard.open(client)
end

local function register()
  login.register(require("games").client)
end

---@param choices string[]
---@return fun(): string[]
local function always(choices)
  return function()
    return choices
  end
end

---@type games.Subcommand[]
local SUBCOMMANDS = {
  { name = "challenge", usage = "challenge [game] [user…]", run = challenge.run },
  { name = "challenges", usage = "challenges", run = find_challenges },
  { name = "solo", usage = "solo [game]", run = solo.run, arg_choices = solo.game_types },
  {
    name = "list",
    usage = "list [ongoing|finished]",
    run = find.run_list,
    arg_choices = always(find.GAME_STATUSES),
  },
  { name = "stats", usage = "stats [user]", run = stats.run_stats },
  { name = "leaderboard", usage = "leaderboard [game]", run = stats.run_leaderboard },
  { name = "next", usage = "next", run = next_game },
  { name = "export", usage = "export", run = export.run },
  { name = "find", usage = "find [source]", run = find.run, arg_choices = always(sources.NAMES) },
  {
    name = "import",
    usage = "import <file> [set] [max]",
    run_offline = puzzle.import,
    arg_choices = puzzle.file_choices,
  },
  { name = "puzzle", usage = "puzzle [set]", run = puzzle.run, arg_choices = puzzles.set_names },
  { name = "register", usage = "register", run_offline = register },
}

---@param name string
---@return games.Subcommand|nil
local function find_subcommand(name)
  return vim.iter(SUBCOMMANDS):find(function(subcommand)
    return subcommand.name == name
  end)
end

---@param client games.Client
---@param fargs string[] as typed, for the label a pending run is reported by
---@param run games.SubcommandRun
---@param args string[]
local function run_connected(client, fargs, run, args)
  local label = table.concat(vim.list_extend({ ":Games" }, fargs), " ")
  login.ensure_connected(client, label, function()
    local me = notify.require_me(client)
    if me then
      run(client, me, args)
    end
  end)
end

---Runs `:Games <fargs>`; with no subcommand it opens the dashboard.
---@param fargs string[]
function M.run(fargs)
  local client = require("games").client
  if not client then
    notify.error('call require("games").setup() first')
    return
  end
  local name = fargs[1]
  if not name then
    run_connected(client, fargs, open_dashboard, {})
    return
  end
  local subcommand = find_subcommand(name)
  if not subcommand then
    notify.error(("unknown subcommand %q"):format(name))
    return
  end
  local args = vim.list_slice(fargs, 2)
  if subcommand.run_offline then
    subcommand.run_offline(args)
    return
  end
  run_connected(client, fargs, assert(subcommand.run), args)
end

---Completes subcommand names, then the first argument where its choices are fixed.
---@param arg_lead string
---@param cmd_line string
---@param cursor_pos integer
---@return string[]
function M.complete(arg_lead, cmd_line, cursor_pos)
  local words = vim.split(cmd_line:sub(1, cursor_pos), "%s+", { trimempty = true })
  local completing = #words + (arg_lead == "" and 1 or 0)
  local candidates = {}
  if completing == 2 then
    candidates = vim.tbl_map(function(subcommand)
      return subcommand.name
    end, SUBCOMMANDS)
  elseif completing == 3 then
    local subcommand = find_subcommand(words[2])
    candidates = subcommand and subcommand.arg_choices and subcommand.arg_choices(arg_lead) or {}
  end
  return vim.tbl_filter(function(candidate)
    return vim.startswith(candidate, arg_lead)
  end, candidates)
end

return M
