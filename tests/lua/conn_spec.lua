local Client = require("games.core.client")
local Conn = require("games.core.conn")
local helpers = require("helpers")

local HEARTBEAT_ARGS = { "--heartbeat", "100ms", "--idle", "300ms" }
local IDLE_MS = 300
local RECONNECT_MIN_MS = 1000

local T = {}

function T.frames_split_across_chunks()
  local lines, rest = Conn.split_frames("", '{"a":1}\n{"b"')
  assert(vim.deep_equal(lines, { '{"a":1}' }), vim.inspect(lines))
  lines, rest = Conn.split_frames(rest, ':2}\n{"c":3}\n')
  assert(vim.deep_equal(lines, { '{"b":2}', '{"c":3}' }), vim.inspect(lines))
  assert(rest == "", rest)
end

function T.typeless_push_does_not_drop_the_rest_of_the_chunk()
  local conn = Conn.new({ host = "127.0.0.1", port = 1, username = "alice", password = "pw" })
  local updates = {}
  conn:on("game.update", function(update)
    table.insert(updates, update.game_id)
  end)
  local attempt = { buffer = "", idle_ms = 60000, watchdog = assert(vim.uv.new_timer()) }
  conn.attempt = attempt
  local ok, err = pcall(
    conn.receive,
    conn,
    attempt,
    nil,
    '{"body":{}}\n{"type":"game.update","body":{"game_id":"g1"}}\n'
  )
  attempt.watchdog:close()
  conn.attempt = nil

  assert(ok, err)
  vim.wait(100, function()
    return #updates > 0
  end)
  assert(vim.deep_equal(updates, { "g1" }), vim.inspect(updates))
end

function T.wire_body_marks_objects_but_keeps_empty_arrays()
  local frame = vim.json.encode(Conn.wire_body({ to = {}, options = {} }))
  assert(frame:find('"options":{}', 1, true) and frame:find('"to":[]', 1, true), frame)
  assert(vim.json.encode(Conn.wire_body(nil)) == "{}")
end

function T.login_and_catalog()
  helpers.with_server(nil, function(env)
    local conn = env.new_conn("alice")
    local log = helpers.record_lifecycle(conn)
    local client = Client.new(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.connected == 1
    end, "login")
    assert(log.connected[1].user == "alice")
    assert(client:me() == "alice")

    local catalog
    client:catalog_list(nil, function(err, result)
      assert(err == nil, vim.inspect(err))
      catalog = result
    end)
    helpers.wait_for(function()
      return catalog ~= nil
    end, "catalog.list reply")
    local ids = vim.tbl_map(function(info)
      return info.id
    end, catalog.games)
    assert(vim.list_contains(ids, "chess"), vim.inspect(ids))
  end)
end

function T.auth_failed_stops_reconnect()
  helpers.with_server(nil, function(env)
    local conn = env.new_conn("alice", "wrong")
    local log = helpers.record_lifecycle(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.disconnected == 1
    end, "disconnect after bad password")
    local disconnect = log.disconnected[1]
    assert(disconnect.code == "auth_failed" and not disconnect.retrying, vim.inspect(disconnect))

    local reattempted = vim.wait(helpers.WAIT_MS, function()
      return #log.disconnected > 1
    end)
    assert(not reattempted, "reconnected after auth_failed")
    assert(#log.connected == 0)
  end)
end

function T.missing_password_fails_without_retrying()
  helpers.with_server(nil, function(env)
    local password_calls = 0
    local conn = env.new_conn("alice", function()
      password_calls = password_calls + 1
      return nil
    end)
    local log = helpers.record_lifecycle(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.disconnected == 1
    end, "disconnect without a password")
    local disconnect = log.disconnected[1]
    assert(disconnect.code == "auth_failed" and not disconnect.retrying, vim.inspect(disconnect))
    assert(disconnect.message:find("no password"), disconnect.message)

    vim.wait(RECONNECT_MIN_MS + 200, function()
      return password_calls > 1
    end)
    assert(password_calls == 1, "retried after a missing password")
  end)
end

function T.reconnects_after_server_restart()
  helpers.with_server(nil, function(env)
    local conn = env.new_conn("alice")
    local log = helpers.record_lifecycle(conn)
    local client = Client.new(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.connected == 1
    end, "first login")

    env.server.restart()
    helpers.wait_for(function()
      return #log.connected == 2
    end, "login after restart")
    assert(#log.disconnected == 1 and log.disconnected[1].retrying, vim.inspect(log.disconnected))
    assert(client:me() == "alice")

    local catalog
    client:catalog_list(nil, function(err, result)
      assert(err == nil, vim.inspect(err))
      catalog = result
    end)
    helpers.wait_for(function()
      return catalog ~= nil
    end, "catalog.list after reconnect")
  end)
end

function T.pending_callbacks_fail_on_disconnect()
  helpers.with_server(nil, function(env)
    local conn = env.new_conn("alice")
    local log = helpers.record_lifecycle(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.connected == 1
    end, "login")

    env.server.pause()
    local reply_err
    conn:request("catalog.list", nil, function(err)
      reply_err = err or { code = "unexpected ok" }
    end)
    env.server.stop()
    helpers.wait_for(function()
      return reply_err ~= nil
    end, "pending callback")
    assert(reply_err.code == "disconnected", vim.inspect(reply_err))
  end)
end

function T.heartbeat_keeps_alive()
  helpers.with_server(HEARTBEAT_ARGS, function(env)
    local conn = env.new_conn("alice")
    local log = helpers.record_lifecycle(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.connected == 1
    end, "login")

    vim.wait(3 * IDLE_MS + 100, function()
      return #log.disconnected > 0
    end)
    assert(#log.disconnected == 0, vim.inspect(log.disconnected))

    local answered = false
    conn:request("ping", nil, function(err)
      answered = err == nil
    end)
    helpers.wait_for(function()
      return answered
    end, "ping reply")
  end)
end

---Answers the n-th login with replies[n] and then hangs up, so the conn logs in again.
---@param replies {type: string, body: table}[]
---@return integer port
---@return fun() stop
local function scripted_login_server(replies)
  local server = assert(vim.uv.new_tcp())
  assert(server:bind("127.0.0.1", 0))
  local logins = 0
  server:listen(8, function()
    local socket = assert(vim.uv.new_tcp())
    server:accept(socket)
    socket:read_start(function(err, chunk)
      if err or not chunk then
        socket:close()
        return
      end
      logins = logins + 1
      local request = vim.json.decode(vim.split(chunk, "\n")[1])
      local reply = vim.tbl_extend("force", { id = request.id }, replies[logins])
      socket:write(vim.json.encode(reply) .. "\n", function()
        socket:close()
      end)
    end)
  end)
  return server:getsockname().port, function()
    server:close()
  end
end

function T.prompted_credentials_are_forgotten_when_a_later_login_is_rejected()
  local port, stop = scripted_login_server({
    { type = "auth.login", body = { user = "alice", heartbeat_ms = 60000 } },
    { type = "error", body = { code = "auth_failed", message = "bad password" } },
  })
  local conn = Conn.new({ host = "127.0.0.1", port = port })
  local log = helpers.record_lifecycle(conn)
  conn:fill_credentials({ username = "alice", password = "rotated-away" })
  conn:connect()
  local rejected = vim.wait(RECONNECT_MIN_MS + helpers.WAIT_MS, function()
    return vim.iter(log.disconnected):any(function(disconnect)
      return not disconnect.retrying
    end)
  end)
  conn:close()
  stop()
  assert(rejected and #log.connected == 1, vim.inspect(log))
  helpers.assert_eq({ "username", "password" }, conn:missing_credentials())
end

function T.a_configured_password_survives_filling_in_the_username()
  helpers.with_server(nil, function(env)
    local conn =
      Conn.new({ host = "127.0.0.1", port = env.server.port, password = helpers.PASSWORD })
    local log = helpers.record_lifecycle(conn)
    conn:fill_credentials({ username = "alice", password = "not-the-configured-one" })
    conn:connect()
    local connected = vim.wait(helpers.WAIT_MS, function()
      return #log.connected == 1
    end)
    conn:close()
    assert(connected, vim.inspect(log))
  end)
end

function T.connect_forgets_the_previous_error()
  local conn = Conn.new({ host = "127.0.0.1", port = 1, username = "alice", password = "pw" })
  conn:connect()
  helpers.wait_for(function()
    return conn:last_error() ~= nil
  end, "the refused connect")
  conn:connect()
  local last_error = conn:last_error()
  conn:close()
  assert(last_error == nil, vim.inspect(last_error))
end

return T
