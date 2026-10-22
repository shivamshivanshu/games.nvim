local Client = require("games.core.client")
local Conn = require("games.core.conn")
local helpers = require("helpers")

local HEARTBEAT_ARGS = { "--heartbeat", "100ms", "--idle", "300ms" }
local IDLE_MS = 300
local RECONNECT_MIN_MS = 1000

---Feeds chunk to conn as if its connection had just read it.
---@param conn games.Conn
---@param chunk string
local function receive(conn, chunk)
  local attempt = { buffer = "", idle_ms = 60000, watchdog = assert(vim.uv.new_timer()) }
  conn.attempt = attempt
  local ok, err = pcall(conn.receive, conn, attempt, nil, chunk)
  attempt.watchdog:close()
  conn.attempt = nil
  assert(ok, err)
end

local UPDATE_FRAME = '{"type":"game.update","body":{"game_id":"g1"}}\n'

local T = {}

function T.a_push_goes_to_the_handlers_subscribed_when_it_arrived()
  local conn = Conn.new({ host = "127.0.0.1", port = 1, username = "alice", password = "pw" })
  local seen = {}
  conn:on("game.update", function()
    table.insert(seen, "early")
  end)
  receive(conn, UPDATE_FRAME)
  conn:on("game.update", function()
    table.insert(seen, "late")
  end)
  helpers.wait_for(function()
    return #seen > 0
  end, "the push")
  vim.wait(20)
  assert(vim.deep_equal(seen, { "early" }), vim.inspect(seen))
end

function T.a_raising_push_handler_reports_its_traceback_and_the_next_still_runs()
  local conn = Conn.new({ host = "127.0.0.1", port = 1, username = "alice", password = "pw" })
  local function failing_update_handler()
    error("handler broke")
  end
  local ran_next = false
  conn:on("game.update", function()
    failing_update_handler()
  end)
  conn:on("game.update", function()
    ran_next = true
  end)
  local raised = {}
  local schedule = vim.schedule
  local function schedule_capturing(fn)
    schedule(function()
      local ok, err = xpcall(fn, debug.traceback)
      if not ok then
        table.insert(raised, err)
      end
    end)
  end
  helpers.with_overrides({ { vim, "schedule", schedule_capturing } }, function()
    receive(conn, UPDATE_FRAME)
    helpers.wait_for(function()
      return ran_next
    end, "the second handler")
  end)
  assert(#raised == 1, vim.inspect(raised))
  assert(raised[1]:find("handler broke", 1, true), raised[1])
  assert(raised[1]:find("failing_update_handler", 1, true), raised[1])
end

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

function T.request_encodes_the_body_and_its_options_as_objects_even_when_empty()
  local conn = Conn.new({ host = "127.0.0.1", port = 1, username = "alice", password = "pw" })
  local frames = {}
  conn.attempt = {
    phase = "authenticated",
    socket = {
      write = function(_, frame)
        table.insert(frames, frame)
      end,
    },
  }
  local function ignore() end
  conn:request("challenge.create", { to = {}, game = "chess", options = {} }, ignore)
  conn:request("ping", nil, ignore)
  conn.attempt = nil

  assert(frames[1]:find('"options":{}', 1, true) and frames[1]:find('"to":[]', 1, true), frames[1])
  assert(frames[2]:find('"body":{}', 1, true), frames[2])
end

function T.close_reports_a_final_disconnect_and_forgets_the_user()
  helpers.with_server(nil, function(env)
    local conn = env.new_conn("alice")
    local log = helpers.record_lifecycle(conn)
    local client = Client.new(conn)
    conn:connect()
    helpers.wait_for(function()
      return client:me() == "alice"
    end, "login")

    conn:close()
    helpers.wait_for(function()
      return #log.disconnected == 1
    end, "the disconnect")
    assert(log.disconnected[1].retrying == false, vim.inspect(log.disconnected))
    helpers.wait_for(function()
      return client:me() == nil
    end, "the user forgotten")
  end)
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
---@param before_reply? fun(n: integer) runs once the n-th login has arrived, before its reply
---@return integer port
---@return fun() stop
---@return games.Envelope[] requests each login request, as it arrived
local function scripted_login_server(replies, before_reply)
  local server = assert(vim.uv.new_tcp())
  assert(server:bind("127.0.0.1", 0))
  local logins = 0
  local requests = {}
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
      table.insert(requests, request)
      if before_reply then
        before_reply(logins)
      end
      local reply = vim.tbl_extend("force", { id = request.id }, replies[logins])
      socket:write(vim.json.encode(reply) .. "\n", function()
        socket:close()
      end)
    end)
  end)
  return server:getsockname().port, function()
    server:close()
  end, requests
end

function T.the_connection_sends_each_frame_at_once_with_nagle_off()
  local port, stop = scripted_login_server({
    { type = "auth.login", body = { user = "alice", heartbeat_ms = 60000 } },
  })
  local new_tcp, nodelay = vim.uv.new_tcp, nil
  local conn_socket_made = false
  helpers.with_overrides({
    {
      vim.uv,
      "new_tcp",
      function()
        local socket = assert(new_tcp())
        if conn_socket_made then
          return socket
        end
        conn_socket_made = true
        return setmetatable({
          nodelay = function(_, enable)
            nodelay = enable
            return socket:nodelay(enable)
          end,
        }, {
          __index = function(_, method)
            return function(_, ...)
              return socket[method](socket, ...)
            end
          end,
        })
      end,
    },
  }, function()
    local conn = Conn.new({ host = "127.0.0.1", port = port, username = "alice", password = "pw" })
    local log = helpers.record_lifecycle(conn)
    conn:connect()
    helpers.wait_for(function()
      return #log.connected == 1
    end, "the login")
    conn:close()
  end)
  stop()
  helpers.assert_eq(true, nodelay)
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

function T.sign_up_replaces_only_the_first_login_and_reconnects_log_in()
  local port, stop, requests = scripted_login_server({
    { type = "ok", body = { user = "erin", heartbeat_ms = 60000 } },
    { type = "ok", body = { user = "erin", heartbeat_ms = 60000 } },
  })
  local conn = Conn.new({ host = "127.0.0.1", port = port })
  local log = helpers.record_lifecycle(conn)
  conn:fill_credentials({ username = "erin", password = "erin-pass" })
  conn:sign_up("sesame")
  conn:connect()
  local reconnected = vim.wait(RECONNECT_MIN_MS + helpers.WAIT_MS, function()
    return #log.connected == 2
  end)
  conn:close()
  stop()
  assert(reconnected, vim.inspect(log))
  local login = { v = 1, username = "erin", password = "erin-pass" }
  helpers.assert_eq(
    {
      { type = "auth.register", body = vim.tbl_extend("error", login, { passphrase = "sesame" }) },
      { type = "auth.login", body = login },
    },
    vim.tbl_map(function(request)
      return { type = request.type, body = request.body }
    end, requests)
  )
end

function T.a_sign_up_armed_while_a_login_is_answered_is_dropped_by_that_login()
  local conn
  local port, stop, requests = scripted_login_server({
    { type = "ok", body = { user = "erin", heartbeat_ms = 60000 } },
    { type = "ok", body = { user = "erin", heartbeat_ms = 60000 } },
  }, function(n)
    if n == 1 then
      conn:sign_up("sesame")
    end
  end)
  conn = Conn.new({ host = "127.0.0.1", port = port, username = "erin", password = "erin-pass" })
  local log = helpers.record_lifecycle(conn)
  conn:connect()
  local reconnected = vim.wait(RECONNECT_MIN_MS + helpers.WAIT_MS, function()
    return #log.connected == 2
  end)
  conn:close()
  stop()
  assert(reconnected, vim.inspect(log))
  helpers.assert_eq(
    { "auth.login", "auth.login" },
    vim.tbl_map(function(request)
      return request.type
    end, requests)
  )
end

function T.a_refused_sign_up_stops_reconnecting_and_forgets_the_prompted_credentials()
  for _, code in ipairs({ "signup_disabled", "passphrase_rejected", "user_exists", "bad_request" }) do
    local port, stop = scripted_login_server({
      { type = "error", body = { code = code, message = "refused" } },
    })
    local conn = Conn.new({ host = "127.0.0.1", port = port })
    local log = helpers.record_lifecycle(conn)
    conn:fill_credentials({ username = "erin", password = "erin-pass" })
    conn:sign_up("sesame")
    conn:connect()
    local refused = vim.wait(helpers.WAIT_MS, function()
      return #log.disconnected == 1
    end)
    conn:close()
    stop()
    assert(refused, code)
    helpers.assert_eq(
      { code = code, message = "refused", retrying = false, sign_up = true },
      log.disconnected[1]
    )
    helpers.assert_eq({ "username", "password" }, conn:missing_credentials())
  end
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
