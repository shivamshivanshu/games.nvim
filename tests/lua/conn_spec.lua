local Client = require("games.core.client")
local Conn = require("games.core.conn")
local helpers = require("helpers")

local HEARTBEAT_ARGS = { "--heartbeat", "100ms", "--idle", "300ms" }
local IDLE_MS = 300

local T = {}

function T.frames_split_across_chunks()
  local lines, rest = Conn.split_frames("", '{"a":1}\n{"b"')
  assert(vim.deep_equal(lines, { '{"a":1}' }), vim.inspect(lines))
  lines, rest = Conn.split_frames(rest, ':2}\n{"c":3}\n')
  assert(vim.deep_equal(lines, { '{"b":2}', '{"c":3}' }), vim.inspect(lines))
  assert(rest == "", rest)
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
    assert(log.disconnected[1].code == "auth_failed", vim.inspect(log.disconnected[1]))

    local reattempted = vim.wait(helpers.WAIT_MS, function()
      return #log.disconnected > 1
    end)
    assert(not reattempted, "reconnected after auth_failed")
    assert(#log.connected == 0)
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

return T
