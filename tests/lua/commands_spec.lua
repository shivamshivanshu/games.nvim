local Client = require("games.core.client")
local commands = require("games.core.commands")
local games = require("games")
local helpers = require("helpers")
local types = require("games.types")

local assert_eq = helpers.assert_eq
local MessageType = types.MessageType
local ErrorCode = types.ErrorCode

local LOGIN = { user = "alice", heartbeat_ms = 15000 }

---@class games.test.PromptStubs
---@field username? string what vim.ui.input answers
---@field password string what inputsecret answers
---@field asked string[] prompts shown, in order

---The plugin's client is a stub-backed one; prompts and notify are stubbed and restored after fn.
---@param credentials games.test.StubCredentials|nil
---@param fn fun(conn: games.test.StubConn, prompts: games.test.PromptStubs)
local function with_stub_client(credentials, fn)
  local conn = helpers.stub_conn(credentials)
  local original = {
    client = games.client,
    input = vim.ui.input,
    inputsecret = vim.fn.inputsecret,
    notify = vim.notify,
  }
  ---@type games.test.PromptStubs
  local prompts = { password = "", asked = {} }
  games.client = Client.new(conn --[[@as games.Conn]])
  vim.ui.input = function(opts, on_confirm)
    table.insert(prompts.asked, opts.prompt)
    on_confirm(prompts.username)
  end
  vim.fn.inputsecret = function(prompt)
    table.insert(prompts.asked, prompt)
    return prompts.password
  end
  vim.notify = function() end
  local ok, err = xpcall(fn, debug.traceback, conn, prompts)
  conn:push("disconnected", { code = ErrorCode.DISCONNECTED, message = "closed", retrying = false })
  games.client = original.client
  vim.ui.input, vim.fn.inputsecret, vim.notify =
    original.input, original.inputsecret, original.notify
  assert(ok, err)
end

---@param conn games.test.StubConn
---@return boolean
local function listed_ongoing(conn)
  return vim.iter(conn.requests):any(function(request)
    return request.type == MessageType.GAME_LIST and request.body ~= nil
  end)
end

local T = {}

function T.connected_runs_immediately()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    conn:push("connected", LOGIN)
    commands.run({ "list" })
    assert(listed_ongoing(conn))
    assert_eq(0, conn.connects)
  end)
end

function T.missing_credentials_are_prompted_then_the_command_runs_once_connected()
  with_stub_client(nil, function(conn, prompts)
    prompts.username, prompts.password = "alice", "alice-pass"
    commands.run({ "list" })
    assert_eq({ "games username: ", "games password: " }, prompts.asked)
    assert_eq({ username = "alice", password = "alice-pass" }, conn.credentials)
    assert_eq(1, conn.connects)
    assert(not listed_ongoing(conn))

    conn:push("connected", LOGIN)
    assert(listed_ongoing(conn))
  end)
end

function T.cancelled_or_empty_prompt_aborts()
  with_stub_client(nil, function(conn, prompts)
    commands.run({ "list" })
    assert_eq({ "games username: " }, prompts.asked)

    prompts.username = "alice"
    commands.run({ "list" })
    assert_eq({}, conn.credentials)
    assert_eq(0, conn.connects)
  end)
end

function T.only_the_newest_pending_command_runs()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    commands.run({ "list" })
    commands.run({ "stats" })
    conn:push("connected", LOGIN)
    assert(not listed_ongoing(conn))
    assert(vim.iter(conn.requests):any(function(request)
      return request.type == MessageType.STATS_GET
    end))
  end)
end

function T.transient_disconnect_keeps_the_command_and_a_fatal_one_drops_it()
  with_stub_client({ username = "alice", password = "pw" }, function(conn)
    commands.run({ "list" })
    conn:push("disconnected", { code = ErrorCode.DISCONNECTED, message = "x", retrying = true })
    conn:push("connected", LOGIN)
    assert(listed_ongoing(conn))

    conn.requests = {}
    conn:push("disconnected", { code = ErrorCode.DISCONNECTED, message = "x", retrying = true })
    commands.run({ "list" })
    conn:push("disconnected", { code = ErrorCode.AUTH_FAILED, message = "bad", retrying = false })
    conn:push("connected", LOGIN)
    assert(not listed_ongoing(conn))
  end)
end

function T.rejected_typed_password_is_forgotten()
  with_stub_client(nil, function(conn, prompts)
    prompts.username, prompts.password = "alice", "wrong"
    commands.run({ "list" })
    conn:push("disconnected", { code = ErrorCode.AUTH_FAILED, message = "bad", retrying = false })
    assert_eq({ username = "alice" }, conn.credentials)
  end)
end

return T
