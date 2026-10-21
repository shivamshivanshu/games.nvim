local describe = require("games.core.describe")
local fs = require("games.core.fs")
local notify = require("games.core.notify")
local registry = require("games.core.registry")
local tables = require("games.core.tables")

local ERROR = vim.log.levels.ERROR
local UUID_PATTERN = "^%x%x%x%x%x%x%x%x%-%x%x%x%x%-%x%x%x%x%-%x%x%x%x%-%x%x%x%x%x%x%x%x%x%x%x%x$"
local LAST_MS_OF_YEAR_9999 = 253402300799999
local EXPORT_NAME_PATTERN = "^[%w_][%w._-]*$"

local File = {
  SUMMARY = "summary.json",
  DIGEST = "summary.sha256",
  FINAL = "final.json",
  EVENTS = "events.jsonl",
  EVENTS_SEQ = "events.seq",
}

---@type table<string, true>
local RESERVED_NAMES = tables.set_of(vim.tbl_values(File))

---@enum games.ArchiveOutcome
local Outcome = {
  ONGOING = "ongoing",
  WRITTEN = "written",
  ALREADY_PRESENT = "already_present",
  REFUSED = "refused",
  FAILED = "failed",
}

---@alias games.ArchiveDone fun(outcome: games.ArchiveOutcome)
---@alias games.ArchiveTally table<games.ArchiveOutcome, integer>

---@class (private) games.ArchivedGame
---@field dir string
---@field label string names the game in notifications
---@field events_seq? integer seq of the last step whose events are in events.jsonl; nil until read
---@field finishing? games.ArchiveDone[] waiting on the finish write in flight

---@class (private) games.ArchiveJob
---@field game games.ArchivedGame
---@field run fun(): games.ArchiveOutcome called inside fs.run
---@field on_done games.ArchiveDone

---Writes each game this client sees under `<root>/<YYYY-MM-DD>/<game>/<game_id>/`, one job at a
---time so a game's files are written in the order its updates arrived.
---@class games.Archive
---@field private root string
---@field private games table<games.GameID, games.ArchivedGame>
---@field private refused table<string, true> game ids nothing more is written for
---@field private queue table<integer, games.ArchiveJob> jobs from queue_head to queue_tail
---@field private queue_head integer
---@field private queue_tail integer
---@field private busy boolean
local Archive = {}
Archive.__index = Archive

Archive.Outcome = Outcome

local function ignore() end

---@param root string absolute
---@return games.Archive
function Archive.new(root)
  return setmetatable({
    root = root,
    games = {},
    refused = {},
    queue = {},
    queue_head = 1,
    queue_tail = 0,
    busy = false,
  }, Archive)
end

---An archive under root that records every update client stores.
---@param client games.Client
---@param root string absolute
---@return games.Archive
function Archive.attach(client, root)
  local archive = Archive.new(root)
  client:on_update(function(update)
    archive:record(update)
  end)
  return archive
end

---@param game games.ArchivedGame
---@param name string
---@return string
local function file_path(game, name)
  return vim.fs.joinpath(game.dir, name)
end

---@param update games.GameUpdate
---@return boolean
local function is_sealed(update)
  return update.summary ~= nil and update.digest ~= nil
end

---An export name other than a plain file name of its own could write outside the game's folder or
---over the archive's own files.
---@param exports? table<string, string>
---@return string|nil
local function unsafe_export(exports)
  for name in pairs(exports or {}) do
    if type(name) ~= "string" or not name:match(EXPORT_NAME_PATTERN) or RESERVED_NAMES[name] then
      return ("export %q is not a plain file name of its own"):format(tostring(name))
    end
  end
end

---Why update cannot be given a folder under the archive root, or nil when it can.
---@param update games.GameUpdate
---@return string|nil
local function unsafe_reason(update)
  if type(update.game_id) ~= "string" or not update.game_id:match(UUID_PATTERN) then
    return "game_id is not a UUID"
  end
  if not registry.is_game_type(update.game) then
    return "game is not a game type name"
  end
  local started_at = update.started_at
  local is_ms = type(started_at) == "number" and started_at == math.floor(started_at)
  if not (is_ms and started_at > 0 and started_at <= LAST_MS_OF_YEAR_9999) then
    return "started_at is not a valid time"
  end
  return unsafe_export(update.exports)
end

---The game's state, or nil when its folder would not be inside the archive root.
---@private
---@param update games.GameUpdate with no unsafe_reason
---@return games.ArchivedGame|nil
function Archive:game_for(update)
  local game = self.games[update.game_id]
  if game then
    return game
  end
  local day = describe.date(update, "%Y-%m-%d")
  local dir = vim.fs.normalize(vim.fs.joinpath(self.root, day, update.game, update.game_id))
  if not vim.startswith(dir, vim.fs.normalize(self.root) .. "/") then
    return nil
  end
  game = { dir = dir, label = ("%s %s"):format(update.game, update.game_id) }
  self.games[update.game_id] = game
  return game
end

---Stops all writes for game_id, notifying the first time.
---@private
---@param game_id any as the server sent it
---@param reason string
---@param on_done games.ArchiveDone
function Archive:refuse(game_id, reason, on_done)
  local key = tostring(game_id)
  if not self.refused[key] then
    self.refused[key] = true
    notify.say(("archive %s: %s; not archiving it"):format(key, reason), ERROR)
  end
  on_done(Outcome.REFUSED)
end

---@private
---@param job games.ArchiveJob
function Archive:enqueue(job)
  self.queue_tail = self.queue_tail + 1
  self.queue[self.queue_tail] = job
  if not self.busy then
    self:run_next()
  end
end

---@private
function Archive:run_next()
  local job = self.queue[self.queue_head]
  self.busy = job ~= nil
  if not job then
    return
  end
  self.queue[self.queue_head] = nil
  self.queue_head = self.queue_head + 1
  fs.run(job.run, function(ok, result)
    if not ok then
      notify.say(("archive %s: %s"):format(job.game.label, result), ERROR)
    end
    local reported, err = pcall(job.on_done, ok and result or Outcome.FAILED)
    if not reported then
      notify.say(("archive %s: %s"):format(job.game.label, err), ERROR)
    end
    self:run_next()
  end)
end

---@param game games.ArchivedGame
---@return integer
local function read_events_seq(game)
  local text = fs.read(file_path(game, File.EVENTS_SEQ))
  return text and tonumber(text) or 0
end

---Appends the events of update unless an earlier run already did, creating the folder first.
---@param game games.ArchivedGame
---@param update games.GameUpdate
local function append_events(game, update)
  if not game.events_seq then
    fs.mkdir_p(game.dir)
    game.events_seq = read_events_seq(game)
  end
  if update.seq <= game.events_seq or #update.events == 0 then
    return
  end
  local lines = vim.tbl_map(vim.json.encode, update.events)
  fs.append(file_path(game, File.EVENTS), table.concat(lines, "\n") .. "\n")
  fs.write_atomic(file_path(game, File.EVENTS_SEQ), tostring(update.seq))
  game.events_seq = update.seq
end

---summary.json goes last, so a matching one means an earlier run wrote everything else. The
---server's exports are written verbatim.
---@param game games.ArchivedGame
---@param update games.GameUpdate sealed, with no unsafe export
---@return games.ArchiveOutcome
local function write_sealed(game, update)
  if fs.read(file_path(game, File.SUMMARY)) == update.summary then
    return Outcome.ALREADY_PRESENT
  end
  fs.write_atomic(file_path(game, File.FINAL), vim.json.encode(update))
  for name, content in pairs(update.exports or {}) do
    fs.write_atomic(file_path(game, name), content)
  end
  fs.write_atomic(file_path(game, File.DIGEST), ("%s  %s\n"):format(update.digest, File.SUMMARY))
  fs.write_atomic(file_path(game, File.SUMMARY), update.summary)
  return Outcome.WRITTEN
end

---Once its files are on disk its state is released; after a failed write it is kept for a retry.
---@private
---@param game games.ArchivedGame
---@param update games.GameUpdate sealed
---@param on_done games.ArchiveDone
function Archive:seal(game, update, on_done)
  if game.finishing then
    table.insert(game.finishing, on_done)
    return
  end
  game.finishing = { on_done }
  self:enqueue({
    game = game,
    run = function()
      append_events(game, update)
      return write_sealed(game, update)
    end,
    on_done = function(outcome)
      local waiting = game.finishing or {}
      game.finishing = nil
      if outcome ~= Outcome.FAILED and self.games[update.game_id] == game then
        self.games[update.game_id] = nil
      end
      for _, done in ipairs(waiting) do
        done(outcome)
      end
    end,
  })
end

---Writes what update adds to its game's folder; on_done gets what was written once it is on disk.
---An update that would write outside the root, or a sealed one whose summary does not match its
---digest, stops all writes for that game.
---@param update games.GameUpdate
---@param on_done? games.ArchiveDone
function Archive:record(update, on_done)
  on_done = on_done or ignore
  if self.refused[tostring(update.game_id)] then
    on_done(Outcome.REFUSED)
    return
  end
  local reason = unsafe_reason(update)
  local game = not reason and self:game_for(update)
  if not game then
    self:refuse(update.game_id, reason or "its folder is outside archive.dir", on_done)
    return
  end
  local sealed = is_sealed(update)
  if sealed and vim.fn.sha256(update.summary) ~= update.digest then
    self:refuse(update.game_id, "summary does not match its digest", on_done)
    return
  end
  if sealed then
    self:seal(game, update, on_done)
    return
  end
  self:enqueue({
    game = game,
    run = function()
      append_events(game, update)
      return Outcome.ONGOING
    end,
    on_done = on_done,
  })
end

---Records each update; on_done gets how many ended with each outcome once all are on disk.
---@param updates games.GameUpdate[]
---@param on_done fun(tally: games.ArchiveTally)
function Archive:backfill(updates, on_done)
  local tally, remaining = {}, #updates
  if remaining == 0 then
    on_done(tally)
    return
  end
  for _, update in ipairs(updates) do
    self:record(update, function(outcome)
      tally[outcome] = (tally[outcome] or 0) + 1
      remaining = remaining - 1
      if remaining == 0 then
        on_done(tally)
      end
    end)
  end
end

return Archive
