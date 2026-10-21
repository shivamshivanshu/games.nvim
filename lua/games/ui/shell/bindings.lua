local controls = require("games.core.controls")
local describe = require("games.core.describe")
local players = require("games.core.players")
local shell_windows = require("games.ui.shell.windows")
local types = require("games.types")

local Control = types.Control

---@class games.KeyBinding: games.KeyHint
---@field aliases? string[] keys mapped to run too, which the footer does not list
---@field run fun(board: games.Board)

---What a binding asks of the shell, which owns the board's drawing and the boards it opens.
---@class games.ShellOps
---@field redraw fun(board: games.Board)
---@field open_next fun(board: games.Board) opens the next game waiting for the player's move

---@class games.ControlKey
---@field control games.Control
---@field key string
---@field wording fun(update: games.GameUpdate): games.ControlWording

---@param label string
---@return fun(): games.ControlWording the same wording for every game
local function labelled(label)
  local wording = { label = label }
  return function()
    return wording
  end
end

---@type games.ControlKey[]
local CONTROL_KEYS = {
  { control = Control.RESIGN, key = "R", wording = describe.resign_wording },
  { control = Control.OFFER_DRAW, key = "D", wording = labelled("offer draw") },
  { control = Control.DECLINE_DRAW, key = "X", wording = labelled("decline draw") },
  { control = Control.ABORT, key = "A", wording = labelled("abort") },
}
local INPUT_KEY = "m"
local FLIP_KEY = "F"
local NEXT_GAME_KEY = "]g"
local CLOSE_KEY = "q"

---@type table<string, true> keys the shell owns whether or not they are active
local CORE_KEYS = vim.iter(CONTROL_KEYS):fold(
  { [INPUT_KEY] = true, [NEXT_GAME_KEY] = true, [CLOSE_KEY] = true },
  function(keys, spec)
    return vim.tbl_extend("error", keys, { [spec.key] = true })
  end
)

local M = {}

---@param board games.Board
---@param game_action games.Action
local function act(board, game_action)
  board.client:game_act(
    { game_id = board.update.game_id, seq = board.update.seq, action = game_action },
    controls.report_failure(board.update, board.me)
  )
end

---What input sends for text: the catalogued action with text in the field the catalogue names,
---which input's action must have.
---@param input games.GameInput
---@param spec games.ActionSpec input's action in the catalogue
---@return fun(text: string): games.Action
local function text_action(input, spec)
  local field = assert(spec.input, ("action %q takes no text"):format(input.kind)).field
  return function(text)
    return { kind = input.kind, [field] = text }
  end
end

---@param board games.Board
---@param prompt string
---@param to_action fun(text: string): games.Action
local function prompt_input(board, prompt, to_action)
  vim.ui.input({ prompt = prompt }, function(text)
    local typed = text and vim.trim(text) or ""
    if typed ~= "" then
      act(board, to_action(typed))
    end
  end)
end

---@param entry games.GameAction|games.KeyBinding
---@return string[] its key, then its aliases
local function keys_of(entry)
  return { entry.key, unpack(entry.aliases or {}) }
end

---@param module games.GameModule
function M.check_action_keys(module)
  local reserved = module.flippable and vim.tbl_extend("error", CORE_KEYS, { [FLIP_KEY] = true })
    or CORE_KEYS
  local seen = {}
  for _, entry in ipairs(module.actions or {}) do
    for _, key in ipairs(keys_of(entry)) do
      assert(not reserved[key], ("game action key %q is reserved by the shell"):format(key))
      assert(not seen[key], ("game action key %q is bound twice"):format(key))
      seen[key] = true
    end
  end
end

---Whether the catalogue lists kind; nothing is, until the catalogue is known.
---@param catalogued? table<games.ActionKind, games.ActionSpec>
---@param kind games.ActionKind
---@return boolean
local function is_catalogued(catalogued, kind)
  return catalogued ~= nil and catalogued[kind] ~= nil
end

---@param entry games.GameAction
---@param update games.GameUpdate
---@return string
local function label_of(entry, update)
  local label = entry.label
  if type(label) == "function" then
    return label(update)
  end
  return label
end

---@param entry games.GameAction|games.GameInput
---@param update games.GameUpdate
---@return boolean
local function is_available(entry, update)
  return not entry.available or entry.available(update)
end

---@param update games.GameUpdate
---@param module games.GameModule
---@param view games.ViewOptions
---@param me games.UserID
---@param catalogued? table<games.ActionKind, games.ActionSpec> the actions the server lists for the game
---@param ops games.ShellOps
---@return games.KeyBinding[]
function M.bindings_for(update, module, view, me, catalogued, ops)
  local ongoing = update.result == nil
  local bindings = {}
  for _, spec in ipairs(CONTROL_KEYS) do
    if vim.tbl_contains(update.controls, spec.control) then
      local wording = spec.wording(update)
      bindings[#bindings + 1] = {
        key = spec.key,
        label = wording.label,
        run = function(board)
          controls.send(
            board.client,
            board.update,
            spec.control,
            wording,
            controls.report_failure(board.update, board.me)
          )
        end,
      }
    end
  end
  local input = module.input
  if
    input
    and players.is_to_act(update, me)
    and is_catalogued(catalogued, input.kind)
    and is_available(input, update)
  then
    ---@cast catalogued -nil
    local to_action = text_action(input, catalogued[input.kind])
    bindings[#bindings + 1] = {
      key = INPUT_KEY,
      label = (input.prompt:gsub(":%s*$", ""):lower()),
      run = function(board)
        prompt_input(board, input.prompt, to_action)
      end,
    }
  end
  for _, entry in ipairs(module.actions or {}) do
    local local_fn, entry_input = entry.local_fn, entry.input
    local game_action = entry.act and entry.act(update, view) or entry.action
    local kind = game_action and game_action.kind or entry_input and entry_input.kind
    if local_fn and is_available(entry, update) then
      bindings[#bindings + 1] = {
        key = entry.key,
        aliases = entry.aliases,
        label = label_of(entry, update),
        run = function(board)
          board.view = local_fn(board.view, board.update)
          ops.redraw(board)
        end,
      }
    elseif ongoing and kind and is_catalogued(catalogued, kind) and is_available(entry, update) then
      ---@cast catalogued -nil
      local to_action = entry_input and text_action(entry_input, catalogued[kind])
      bindings[#bindings + 1] = {
        key = entry.key,
        aliases = entry.aliases,
        label = label_of(entry, update),
        run = function(board)
          if entry_input then
            prompt_input(board, entry_input.prompt, assert(to_action))
          else
            act(board, game_action --[[@as games.Action]])
          end
        end,
      }
    end
  end
  if module.flippable then
    bindings[#bindings + 1] = {
      key = FLIP_KEY,
      label = "flip",
      run = function(board)
        board.view = vim.tbl_extend("force", board.view, { flipped = not board.view.flipped })
        ops.redraw(board)
      end,
    }
  end
  bindings[#bindings + 1] = {
    key = NEXT_GAME_KEY,
    label = "next game",
    run = ops.open_next,
  }
  bindings[#bindings + 1] = { key = CLOSE_KEY, label = "close", run = shell_windows.close_window }
  return bindings
end

---@param board games.Board
---@param bindings games.KeyBinding[]
function M.map_keys(board, bindings)
  local by_key = {}
  for _, binding in ipairs(bindings) do
    for _, key in ipairs(keys_of(binding)) do
      by_key[key] = binding
      local mapped = board.bindings[key]
      if not (mapped and mapped.label == binding.label) then
        vim.keymap.set("n", key, function()
          board.bindings[key].run(board)
        end, { buffer = board.buf, nowait = true, desc = "games: " .. binding.label })
      end
    end
  end
  for key in pairs(board.bindings) do
    if not by_key[key] then
      vim.keymap.del("n", key, { buffer = board.buf })
    end
  end
  board.bindings = by_key
end

return M
