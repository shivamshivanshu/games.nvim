local Conn = require("games.core.conn")
local types = require("games.types")

local MessageType = types.MessageType
local ErrorCode = types.ErrorCode
local ChallengeState = types.ChallengeState

---@alias games.UpdateHandler fun(update: games.GameUpdate, is_new: boolean)
---@alias games.ChallengeHandler fun(challenge: games.Challenge, is_new: boolean)

---@class games.Client
---@field conn games.Conn
---@field package user? games.UserID
---@field private updates table<games.GameID, games.GameUpdate>
---@field private received_ms table<games.GameID, integer>
---@field private challenges table<games.ChallengeID, games.Challenge>
---@field private update_handlers games.UpdateHandler[]
---@field private challenge_handlers games.ChallengeHandler[]
---@field catalog_list fun(self: games.Client, body: nil, cb?: fun(err: games.ErrorBody|nil, result: games.CatalogResult|nil))
---@field user_list fun(self: games.Client, body: nil, cb?: fun(err: games.ErrorBody|nil, result: games.UserListResult|nil))
---@field challenge_accept fun(self: games.Client, body: games.ChallengeRef, cb?: games.ReplyCallback)
---@field challenge_decline fun(self: games.Client, body: games.ChallengeRef, cb?: games.ReplyCallback)
---@field challenge_cancel fun(self: games.Client, body: games.ChallengeRef, cb?: games.ReplyCallback)
---@field challenge_list fun(self: games.Client, body: nil, cb?: fun(err: games.ErrorBody|nil, result: games.ChallengeListResult|nil))
---@field game_list fun(self: games.Client, body: games.GameListRequest|nil, cb?: fun(err: games.ErrorBody|nil, result: games.GameListResult|nil))
---@field game_snapshot fun(self: games.Client, body: games.GameSnapshotRequest, cb?: fun(err: games.ErrorBody|nil, result: games.GameUpdate|nil))
---@field game_act fun(self: games.Client, body: games.GameActRequest, cb?: fun(err: games.ErrorBody|nil, result: games.SeqResult|nil))
---@field game_control fun(self: games.Client, body: games.GameControlRequest, cb?: fun(err: games.ErrorBody|nil, result: games.SeqResult|nil))
---@field stats_get fun(self: games.Client, body: nil, cb?: fun(err: games.ErrorBody|nil, result: games.StatsResult|nil))
local Client = {}
Client.__index = Client

---@alias games.ReplyHook fun(self: games.Client, body: table|nil, err: games.ErrorBody|nil, result: table|nil)

---Builds the typed wrapper for one request type; on_reply lets the cache see the reply before cb.
---@param message_type games.MessageType
---@param on_reply? games.ReplyHook
---@return function
local function request_fn(message_type, on_reply)
  return function(self, body, cb)
    self.conn:request(message_type, body, function(err, result)
      if on_reply then
        on_reply(self, body, err, result)
      end
      if cb then
        cb(err, result)
      end
    end)
  end
end

---@type games.ReplyHook
local function store_listed_games(self, _, _, result)
  for _, update in ipairs(result and result.games or {}) do
    self:store(update)
  end
end

---@type games.ReplyHook
local function store_snapshot(self, _, _, result)
  if result then
    self:store(result --[[@as games.GameUpdate]])
  end
end

---@type games.ReplyHook
local function snapshot_if_stale(self, body, err)
  if err and err.code == ErrorCode.STALE and body then
    self:game_snapshot({ game_id = body.game_id })
  end
end

---@type games.ReplyHook
local function cache_challenges(self, _, _, result)
  if result then
    self:replace_challenges(result.challenges)
  end
end

Client.catalog_list = request_fn(MessageType.CATALOG_LIST)
Client.user_list = request_fn(MessageType.USER_LIST)
Client.challenge_accept = request_fn(MessageType.CHALLENGE_ACCEPT)
Client.challenge_decline = request_fn(MessageType.CHALLENGE_DECLINE)
Client.challenge_cancel = request_fn(MessageType.CHALLENGE_CANCEL)
Client.challenge_list = request_fn(MessageType.CHALLENGE_LIST, cache_challenges)
Client.game_list = request_fn(MessageType.GAME_LIST, store_listed_games)
Client.game_snapshot = request_fn(MessageType.GAME_SNAPSHOT, store_snapshot)
Client.game_act = request_fn(MessageType.GAME_ACT, snapshot_if_stale)
Client.game_control = request_fn(MessageType.GAME_CONTROL, snapshot_if_stale)
Client.stats_get = request_fn(MessageType.STATS_GET)

local send_challenge_create = request_fn(MessageType.CHALLENGE_CREATE)

---@param body games.ChallengeCreateRequest
---@param cb? fun(err: games.ErrorBody|nil, result: games.ChallengeCreateResult|nil)
function Client:challenge_create(body, cb)
  local wire_body = vim.tbl_extend("force", body, { options = Conn.as_json_object(body.options) })
  send_challenge_create(self, wire_body, cb)
end

---@param conn games.Conn
---@return games.Client
function Client.new(conn)
  local self = setmetatable({
    conn = conn,
    updates = {},
    received_ms = {},
    challenges = {},
    update_handlers = {},
    challenge_handlers = {},
  }, Client)
  conn:on(MessageType.GAME_UPDATE, function(update)
    self:receive_update(update)
  end)
  conn:on(MessageType.CHALLENGE_CHANGED, function(challenge)
    self:receive_challenge(challenge)
  end)
  conn:on("connected", function(result)
    self.user = result.user
    self:resync()
  end)
  conn:on("disconnected", function()
    self.user = nil
  end)
  return self
end

---The logged-in user, or nil while disconnected.
---@return games.UserID|nil
function Client:me()
  return self.user
end

---@param game_id games.GameID
---@return games.GameUpdate|nil
function Client:game(game_id)
  return self.updates[game_id]
end

---Loop time (`vim.uv.now()`, ms) when the cached update for game_id was stored.
---@param game_id games.GameID
---@return integer|nil
function Client:received_at(game_id)
  return self.received_ms[game_id]
end

---@return games.GameUpdate[]
function Client:games()
  return vim.tbl_values(self.updates)
end

---@param fn games.UpdateHandler called with is_new = true the first time a game_id is seen
function Client:on_update(fn)
  table.insert(self.update_handlers, fn)
end

---@param fn games.ChallengeHandler called with is_new = true the first time a challenge_id is seen
function Client:on_challenge(fn)
  table.insert(self.challenge_handlers, fn)
end

---Caches update unless the cache already holds this seq or a later one.
---@package
---@param update games.GameUpdate
function Client:store(update)
  local cached = self.updates[update.game_id]
  if cached and cached.seq >= update.seq then
    return
  end
  self.updates[update.game_id] = update
  self.received_ms[update.game_id] = vim.uv.now()
  for _, fn in ipairs(self.update_handlers) do
    fn(update, cached == nil)
  end
end

---A push that skips a seq means we missed steps, so the snapshot (not the push) becomes the truth.
---@package
---@param update games.GameUpdate
function Client:receive_update(update)
  local cached = self.updates[update.game_id]
  if cached and update.seq > cached.seq + 1 then
    self:game_snapshot({ game_id = update.game_id })
    return
  end
  self:store(update)
end

---@package
---@param challenge games.Challenge
function Client:receive_challenge(challenge)
  local is_new = self.challenges[challenge.challenge_id] == nil
  self.challenges[challenge.challenge_id] = challenge.state == ChallengeState.PENDING and challenge
    or nil
  for _, fn in ipairs(self.challenge_handlers) do
    fn(challenge, is_new)
  end
end

---@package
---@param challenges games.Challenge[]
function Client:replace_challenges(challenges)
  local previous = self.challenges
  self.challenges = {}
  for _, challenge in ipairs(challenges) do
    self.challenges[challenge.challenge_id] = challenge
    if not previous[challenge.challenge_id] then
      for _, fn in ipairs(self.challenge_handlers) do
        fn(challenge, true)
      end
    end
  end
end

---Games that finished while we were offline drop out of the ongoing list, so snapshot them.
---@package
---@param listed games.GameUpdate[]
function Client:snapshot_unlisted(listed)
  local listed_ids = {}
  for _, update in ipairs(listed) do
    listed_ids[update.game_id] = true
  end
  for game_id, update in pairs(self.updates) do
    if update.result == nil and not listed_ids[game_id] then
      self:game_snapshot({ game_id = game_id })
    end
  end
end

---@package
function Client:resync()
  self:game_list(nil, function(_, result)
    if result then
      self:snapshot_unlisted(result.games)
    end
  end)
  self:challenge_list(nil)
end

return Client
