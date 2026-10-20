local helpers = require("helpers")
local json = require("games.core.json")
local types = require("games.types")

local assert_eq = helpers.assert_eq

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
local SCHEMA_DIR = REPO_ROOT .. "/docs/schema"

---The Lua class of each schema definition whose name differs; a definition named
---like a games.* class needs no entry.
local PROTOCOL_CLASS = {
  ActionInputWire = "games.ActionInput",
  ActionWire = "games.ActionSpec",
  GameInfoWire = "games.GameInfo",
  OptionWire = "games.GameOption",
  Player = "games.PlayerSeat",
  PlayersWire = "games.PlayerRange",
}

---Per game, the Lua class of each definition its module annotates.
local GAME_CLASS = {
  checkers = {
    MovedEvent = "games.CheckersMovedEvent",
    View = "games.CheckersView",
  },
  chess = {
    MoveSquares = "games.ChessSquares",
    MovedEvent = "games.ChessMovedEvent",
    View = "games.ChessView",
  },
  tictactoe = {
    PlacedEvent = "games.TicTacToePlacedEvent",
    View = "games.TicTacToeView",
  },
  connect4 = {
    Cell = "games.Connect4Cell",
    DroppedEvent = "games.Connect4DroppedEvent",
    View = "games.Connect4View",
  },
}

---Enums the server sends that the client adds no value to.
local CLIENT_ONLY = { ErrorCode = { types.ErrorCode.DISCONNECTED } }

local function read_schema(name)
  return json.decode(helpers.read_file(SCHEMA_DIR .. "/" .. name))
end

---@class (private) LuaField
---@field optional boolean
---@field type string the annotation's type expression

---@class (private) LuaClass
---@field parent? string
---@field fields table<string, LuaField>

---@class (private) LuaAnnotations
---@field classes table<string, LuaClass>
---@field aliases table<string, string> each `---@alias` name's type expression
---@field enums table<string, true>

local VISIBILITY = { public = true, protected = true, private = true, package = true }

---The type expression at the start of text: `table<...>` whole, then any `[]`.
---@param text string
---@return string
local function type_expression(text)
  local expression = text:match("^table%b<>") or text:match("^%S+")
  return expression .. (text:sub(#expression + 1):match("^%[%]") or "")
end

---The name, optionality and type of a `---@field [visibility] name[?] type` line.
---@param line string
---@return string|nil name
---@return LuaField|nil
local function parse_field(line)
  local rest = line:match("^%-%-%-@field%s+(.*)")
  if not rest then
    return nil
  end
  local first, after = rest:match("^(%S+)%s+(.*)")
  if first and VISIBILITY[first] then
    rest = after
  end
  local name, optional, text = rest:match("^([%w_]+)(%??)%s+(.*)")
  if not name then
    return nil
  end
  return name, { optional = optional == "?", type = type_expression(text) }
end

---Every `---@class`, `---@alias` and `---@enum` annotated under lua/.
---@return LuaAnnotations
local function lua_annotations()
  local found = { classes = {}, aliases = {}, enums = {} }
  local current
  for _, file in ipairs(vim.fn.globpath(REPO_ROOT .. "/lua", "**/*.lua", false, true)) do
    for line in io.lines(file) do
      local class, parent = line:match("^%-%-%-@class ([%w%.]+):? ?([%w%.]*)")
      local alias, aliased = line:match("^%-%-%-@alias ([%w%.]+)%s+(.*)")
      local enum = line:match("^%-%-%-@enum ([%w%.]+)")
      local field, spec = parse_field(line)
      if class then
        current = { parent = parent ~= "" and parent or nil, fields = {} }
        found.classes[class] = current
      elseif field and current then
        current.fields[field] = spec
      elseif alias then
        found.aliases[alias] = type_expression(aliased)
      elseif enum then
        found.enums[enum] = true
      elseif not line:match("^%-%-%-") then
        current = nil
      end
    end
  end
  return found
end

---Every field of class and of the classes it inherits from.
---@param classes table<string, LuaClass>
---@param class LuaClass
---@return table<string, LuaField>
local function all_fields(classes, class)
  local fields = {}
  while class do
    fields = vim.tbl_extend("keep", fields, class.fields)
    class = class.parent and classes[class.parent]
  end
  return fields
end

---The JSON kind a Lua type expression decodes from: integer, number, string,
---boolean, array or object, or any when it says nothing narrower.
---@param annotations LuaAnnotations
---@param expression string
---@return string
local function lua_kind(annotations, expression)
  local scalar = { integer = "integer", number = "number", string = "string", boolean = "boolean" }
  if expression:match("%[%]$") then
    return "array"
  elseif expression:match("^table") or annotations.classes[expression] then
    return "object"
  elseif scalar[expression] or expression:match('^"') then
    return scalar[expression] or "string"
  elseif annotations.enums[expression] then
    return "string"
  elseif annotations.aliases[expression] then
    return lua_kind(annotations, annotations.aliases[expression])
  end
  return "any"
end

local function nullable(property)
  for _, alternative in ipairs(type(property) == "table" and property.oneOf or {}) do
    if alternative.type == "null" then
      return true
    end
  end
  return false
end

---The JSON kind a schema allows, as lua_kind names them; resolve gives the
---definition a $ref points to.
---@param property table|boolean
---@param resolve fun(ref: string): table
---@return string
local function wire_kind(property, resolve)
  if type(property) ~= "table" then
    return "any"
  elseif property.oneOf then
    local typed = vim.tbl_filter(function(alternative)
      return alternative.type ~= "null"
    end, property.oneOf)
    return #typed == 1 and wire_kind(typed[1], resolve) or "any"
  elseif property["$ref"] then
    return wire_kind(resolve(property["$ref"]), resolve)
  elseif property.const ~= nil then
    return type(property.const) == "string" and "string" or "any"
  end
  return property.type or "any"
end

---Each mismatch between the schema's object definitions and their Lua
---classes, both ways: a missing class, a wire field the class lacks, a class
---field the wire does not send, or a field whose optionality or JSON kind
---differs from the wire's.
---@param defs table<string, table>
---@param class_of fun(def_name: string): string|nil
---@param annotations LuaAnnotations
---@param resolve fun(ref: string): table
---@return string[]
local function mismatches(defs, class_of, annotations, resolve)
  local found = {}
  local function mismatch(format, ...)
    table.insert(found, format:format(...))
  end
  for def_name, def in pairs(defs) do
    local class_name = class_of(def_name)
    local class = class_name and annotations.classes[class_name]
    if def.properties and next(def.properties) and class_name and not class then
      mismatch("%s: no class %s", def_name, class_name)
    elseif class then
      local fields = all_fields(annotations.classes, class)
      local required = {}
      for _, name in ipairs(def.required or {}) do
        required[name] = true
      end
      for name, property in pairs(def.properties) do
        local field = fields[name]
        local optional = not required[name] or nullable(property)
        local want, got = wire_kind(property, resolve), field and lua_kind(annotations, field.type)
        if not field then
          mismatch("%s.%s: no field", class_name, name)
        elseif field.optional ~= optional then
          mismatch("%s.%s: optional is %s, want %s", class_name, name, field.optional, optional)
        elseif want ~= "any" and got ~= "any" and want ~= got then
          mismatch("%s.%s: %s, want %s", class_name, name, field.type, want)
        end
      end
      for name in pairs(fields) do
        if def.properties[name] == nil then
          mismatch("%s.%s: not on the wire", class_name, name)
        end
      end
    end
  end
  table.sort(found)
  return found
end

local function sorted_values(enum, except)
  local values = vim.tbl_filter(function(v)
    return not vim.tbl_contains(except or {}, v)
  end, vim.tbl_values(enum))
  table.sort(values)
  return values
end

local T = {}

function T.every_wire_enum_matches_its_lua_enum()
  local defs = read_schema("protocol.json")["$defs"]
  for _, name in ipairs({
    "MessageType",
    "ErrorCode",
    "ChallengeState",
    "GameStatus",
    "Control",
    "OptionType",
    "OptionRole",
    "ResultKind",
  }) do
    local wire = vim.deepcopy(defs[name].enum)
    table.sort(wire)
    assert_eq({ name, wire }, { name, sorted_values(types[name], CLIENT_ONLY[name]) })
  end
  local core_reasons = vim.deepcopy(defs.ResultReason.examples)
  table.sort(core_reasons)
  assert_eq(core_reasons, sorted_values(types.ResultReason))
  assert_eq(read_schema("protocol.json")["x-version"], types.PROTOCOL_VERSION)
end

function T.every_wire_field_is_annotated_as_the_wire_sends_it()
  local annotations = lua_annotations()
  local protocol = read_schema("protocol.json")
  local documents = { protocol = protocol }
  ---@param document table the schema the ref appears in
  local function resolver(document)
    return function(ref)
      local file, name = ref:match("^(.-)#/%$defs/(.+)$")
      return (file == "" and document or documents.protocol)["$defs"][name]
    end
  end
  assert_eq(
    {},
    mismatches(protocol["$defs"], function(def_name)
      return PROTOCOL_CLASS[def_name] or "games." .. def_name
    end, annotations, resolver(protocol))
  )
  for game, class_of in pairs(GAME_CLASS) do
    local schema = read_schema("games/" .. game .. ".json")
    assert_eq(
      {},
      mismatches(schema["$defs"], function(def_name)
        return class_of[def_name]
      end, annotations, resolver(schema))
    )
  end
end

return T
