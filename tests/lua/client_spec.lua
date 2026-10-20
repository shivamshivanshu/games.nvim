local Client = require("games.core.client")

---@class games.test.StubConn
---@field handlers table<string, fun(body: table)[]>
---@field requests {type: string, body: table|nil, cb: fun(err: games.ErrorBody|nil, result: table|nil)}[]

---@return games.test.StubConn
local function stub_conn()
  local conn = { handlers = {}, requests = {} }
  function conn:on(event, fn)
    self.handlers[event] = self.handlers[event] or {}
    table.insert(self.handlers[event], fn)
  end
  function conn:request(message_type, body, cb)
    table.insert(self.requests, { type = message_type, body = body, cb = cb })
  end
  function conn:push(event, body)
    for _, fn in ipairs(self.handlers[event] or {}) do
      fn(body)
    end
  end
  return conn
end

---@param seq integer
---@return games.GameUpdate
local function update(seq)
  return {
    game_id = "g1",
    game = "chess",
    seq = seq,
    players = {},
    to_act = {},
    controls = {},
    events = {},
  }
end

local T = {}

function T.seq_gap_snapshots_instead_of_applying()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  conn:push("game.update", update(1))
  conn:push("game.update", update(3))

  assert(client:game("g1").seq == 1)
  assert(
    #conn.requests == 1 and conn.requests[1].type == "game.snapshot",
    vim.inspect(conn.requests)
  )
  assert(conn.requests[1].body.game_id == "g1")
  conn.requests[1].cb(nil, update(3))
  assert(client:game("g1").seq == 3)
end

function T.stale_error_snapshots_and_reports()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  local reported
  client:game_act(
    { game_id = "g1", seq = 0, action = { kind = "move", move = "e4" } },
    function(err)
      reported = err
    end
  )
  conn.requests[1].cb({ code = "stale", message = "seq 0 is stale" }, nil)

  assert(reported.code == "stale")
  assert(conn.requests[2].type == "game.snapshot" and conn.requests[2].body.game_id == "g1")
end

function T.received_at_stamps_each_cached_update()
  local conn = stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  assert(client:received_at("g1") == nil)

  conn:push("game.update", update(1))
  local first = client:received_at("g1")
  vim.wait(20, function()
    return vim.uv.now() > first
  end)
  conn:push("game.update", update(1))
  assert(client:received_at("g1") == first, "a duplicate seq must not restamp")
  conn:push("game.update", update(2))
  assert(client:received_at("g1") > first)
end

function T.connected_resyncs_games_and_challenges()
  local conn = stub_conn()
  Client.new(conn --[[@as games.Conn]])
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })

  local types = vim.tbl_map(function(request)
    return request.type
  end, conn.requests)
  assert(vim.deep_equal(types, { "game.list", "challenge.list" }), vim.inspect(types))
end

return T
