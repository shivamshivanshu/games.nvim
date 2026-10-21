local Archive = require("games.core.archive")
local Client = require("games.core.client")
local helpers = require("helpers")
local json = require("games.core.json")

local assert_eq = helpers.assert_eq
local read_file = helpers.read_file
local Outcome = Archive.Outcome

local GAME_ID = "0b6c6f3e-5d4a-4f7e-9c51-2a8e3d7f1b20"
local STARTED_AT = 1792584000000
local DAY = os.date("%Y-%m-%d", STARTED_AT / 1000)
local FOOLS_MATE = { "f3", "e5", "g4", "Qh4#" }
local SEATS = { "white", "black" }
local SUMMARY = (
  '{"game_id":"%s","game":"chess","started_at":%d,"finished_at":%d,'
  .. '"reason":"checkmate","outcomes":[{"user":"alice","score":-1},{"user":"bob","score":1}]}'
):format(GAME_ID, STARTED_AT, STARTED_AT + 60000)
local DIGEST_LINE = vim.fn.sha256(SUMMARY) .. "  summary.json\n"
local PGN = '[Event "games.nvim"]\n\n1. f3 e5 2. g4 Qh4# 0-1\n'

---Fool's mate after seq plies, with the last ply's event.
---@param seq integer
---@return games.GameUpdate
local function ongoing(seq)
  return {
    game_id = GAME_ID,
    game = "chess",
    seq = seq,
    players = { { user = "alice", seat = "white" }, { user = "bob", seat = "black" } },
    to_act = {},
    controls = {},
    events = { { kind = "moved", by = SEATS[(seq - 1) % 2 + 1], san = FOOLS_MATE[seq] } },
    view = {
      fen = "unused",
      history_san = vim.list_slice(FOOLS_MATE, 1, seq),
      legal_moves = {},
      claimable_draw = false,
    },
    started_at = STARTED_AT,
  }
end

---@return games.GameUpdate
local function finishing_broadcast()
  local update = ongoing(#FOOLS_MATE)
  update.result = { kind = "decisive", winners = { "bob" }, reason = "checkmate" }
  update.finished_at = STARTED_AT + 60000
  update.exports = { ["game.pgn"] = PGN }
  return helpers.seal(update, SUMMARY)
end

---The finished game as any read after its finishing broadcast shows it.
---@return games.GameUpdate
local function later_read()
  local update = finishing_broadcast()
  update.events = {}
  return update
end

---@param root string
---@param name string
---@return string
local function game_file(root, name)
  return vim.fs.joinpath(root, DAY, "chess", GAME_ID, name)
end

---@param archive games.Archive
---@param update games.GameUpdate
---@return games.ArchiveOutcome
local function record(archive, update)
  local outcome
  archive:record(update, function(result)
    outcome = result
  end)
  helpers.wait_for(function()
    return outcome ~= nil
  end, "the archive write")
  return outcome
end

---@param root string
---@return string[] the san of each archived event
local function archived_moves(root)
  local lines =
    vim.split(read_file(game_file(root, "events.jsonl")) or "", "\n", { trimempty = true })
  return vim.tbl_map(function(line)
    return vim.json.decode(line).san
  end, lines)
end

local T = {}

function T.finishing_broadcast_archives_the_whole_game()
  helpers.with_temp_dir(function(root)
    local conn = helpers.stub_conn()
    Archive.attach(Client.new(conn --[[@as games.Conn]]), root)
    for seq = 1, #FOOLS_MATE - 1 do
      conn:push("game.update", ongoing(seq))
    end
    conn:push("game.update", finishing_broadcast())
    helpers.wait_for(function()
      return read_file(game_file(root, "summary.json")) ~= nil
    end, "summary.json")

    assert_eq(SUMMARY, read_file(game_file(root, "summary.json")))
    assert_eq(DIGEST_LINE, read_file(game_file(root, "summary.sha256")))
    assert_eq(finishing_broadcast(), json.decode(read_file(game_file(root, "final.json")) or ""))
    assert_eq(FOOLS_MATE, archived_moves(root))
    assert_eq(PGN, read_file(game_file(root, "game.pgn")))
  end)
end

function T.digest_mismatch_writes_nothing_and_notifies_once()
  helpers.with_temp_dir(function(root)
    helpers.capture_notifications(function(notified)
      local archive = Archive.new(root)
      local tampered =
        vim.tbl_extend("force", finishing_broadcast(), { digest = vim.fn.sha256("x") })
      assert_eq(Outcome.REFUSED, record(archive, tampered))
      assert_eq(Outcome.REFUSED, record(archive, tampered))
      assert_eq(Outcome.REFUSED, record(archive, ongoing(1)))

      assert_eq(1, #notified)
      assert(vim.uv.fs_stat(root) == nil, "the archive wrote to " .. root)
    end)
  end)
end

function T.events_resent_after_a_reconnect_are_not_appended_again()
  helpers.with_temp_dir(function(root)
    local archive = Archive.new(root)
    for _, seq in ipairs({ 1, 2, 2, 1, 3 }) do
      record(archive, ongoing(seq))
    end
    assert_eq({ "f3", "e5", "g4" }, archived_moves(root))
  end)
end

function T.a_restarted_archive_continues_after_the_last_written_event()
  helpers.with_temp_dir(function(root)
    local before = Archive.new(root)
    record(before, ongoing(1))
    record(before, ongoing(2))
    local after = Archive.new(root)
    record(after, ongoing(2))
    record(after, ongoing(3))
    assert_eq({ "f3", "e5", "g4" }, archived_moves(root))
  end)
end

function T.a_later_read_of_an_unseen_game_writes_its_final_view_and_exports()
  helpers.with_temp_dir(function(root)
    local archive = Archive.new(root)
    assert_eq(Outcome.WRITTEN, record(archive, later_read()))
    assert_eq(SUMMARY, read_file(game_file(root, "summary.json")))
    assert_eq(DIGEST_LINE, read_file(game_file(root, "summary.sha256")))
    assert_eq(later_read(), json.decode(read_file(game_file(root, "final.json")) or ""))
    assert_eq(PGN, read_file(game_file(root, "game.pgn")))
  end)
end

function T.a_finished_game_is_written_once()
  helpers.with_temp_dir(function(root)
    assert_eq(Outcome.WRITTEN, record(Archive.new(root), finishing_broadcast()))
    assert_eq(Outcome.ALREADY_PRESENT, record(Archive.new(root), later_read()))
    assert(read_file(game_file(root, "final.json")), "final.json was lost")
  end)
end

function T.a_sealed_game_is_released_and_a_later_record_of_it_reads_the_disk()
  helpers.with_temp_dir(function(root)
    local archive = Archive.new(root)
    for seq = 1, #FOOLS_MATE - 1 do
      record(archive, ongoing(seq))
    end
    assert_eq(Outcome.WRITTEN, record(archive, finishing_broadcast()))
    assert_eq({}, archive.games)

    assert_eq(Outcome.ALREADY_PRESENT, record(archive, finishing_broadcast()))
    assert_eq(FOOLS_MATE, archived_moves(root))
  end)
end

function T.a_failed_seal_is_written_by_the_next_finished_update()
  helpers.with_temp_dir(function(root)
    local archive = Archive.new(root)
    for seq = 1, #FOOLS_MATE - 1 do
      record(archive, ongoing(seq))
    end
    local summary_path = game_file(root, "summary.json")
    vim.fn.mkdir(summary_path, "p")
    helpers.capture_notifications(function()
      assert_eq(Outcome.FAILED, record(archive, finishing_broadcast()))
    end)
    vim.fn.delete(summary_path, "d")

    assert_eq(Outcome.WRITTEN, record(archive, later_read()))
    assert_eq(later_read(), json.decode(read_file(game_file(root, "final.json")) or ""))
    assert_eq(FOOLS_MATE, archived_moves(root))
  end)
end

function T.updates_that_would_leave_the_root_are_refused()
  local cases = {
    { field = "game_id", value = "../../../escaped" },
    { field = "game_id", value = GAME_ID .. "/x" },
    { field = "game_id", value = "" },
    { field = "game_id", value = nil },
    { field = "game", value = ".." },
    { field = "game", value = "chess/../.." },
    { field = "game", value = "" },
    { field = "started_at", value = nil },
    { field = "started_at", value = 1e300 },
    { field = "started_at", value = -1 },
    { field = "started_at", value = 1.5 },
    { field = "exports", value = { ["../escaped"] = "x" } },
    { field = "exports", value = { ["sub/game.pgn"] = "x" } },
    { field = "exports", value = { [".hidden"] = "x" } },
    { field = "exports", value = { ["summary.json"] = "{}" } },
  }
  for _, case in ipairs(cases) do
    helpers.with_temp_dir(function(root)
      helpers.capture_notifications(function(notified)
        local update = ongoing(1)
        update[case.field] = case.value
        local archive = Archive.new(root)
        assert_eq(Outcome.REFUSED, record(archive, update))
        assert_eq(Outcome.REFUSED, record(archive, update))
        assert_eq({ case = case, notified = 1 }, { case = case, notified = #notified })
        assert(vim.uv.fs_stat(root) == nil, "wrote for " .. vim.inspect(case))
      end)
    end)
  end
end

function T.summary_sha256_checks_with_sha256sum()
  if vim.fn.executable("sha256sum") == 0 then
    return
  end
  helpers.with_temp_dir(function(root)
    record(Archive.new(root), finishing_broadcast())
    local check = vim
      .system({ "sha256sum", "-c", "summary.sha256" }, { cwd = vim.fs.dirname(game_file(root, "x")) })
      :wait()
    assert_eq(
      { code = 0, stdout = "summary.json: OK\n" },
      { code = check.code, stdout = check.stdout }
    )
  end)
end

return T
