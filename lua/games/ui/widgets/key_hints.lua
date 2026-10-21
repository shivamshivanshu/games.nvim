local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local text_util = require("games.ui.text")

local Group = highlights.Group

local GAP = "  "

---@class games.KeyHint
---@field key string
---@field label string

---@class (private) games.KeyHintGroup
---@field keys string[]
---@field label string

---Adjacent hints that share a label, as one group each.
---@param hints games.KeyHint[]
---@return games.KeyHintGroup[]
local function grouped(hints)
  local groups = {}
  for _, hint in ipairs(hints) do
    local last = groups[#groups]
    if last and last.label == hint.label then
      last.keys[#last.keys + 1] = hint.key
    else
      groups[#groups + 1] = { keys = { hint.key }, label = hint.label }
    end
  end
  return groups
end

---@param a string
---@param b string
---@return boolean
local function follows(a, b)
  return a:match("^%d$") ~= nil and b:match("^%d$") ~= nil and b:byte() == a:byte() + 1
end

---@param part string
---@return boolean
local function is_letter(part)
  return part:match("^%a$") ~= nil
end

---Keys as written in one hint: runs of consecutive digits become `first-last` and adjacent
---letters run together, e.g. `1-3,5`, `hjkl`, `h,<Left>`.
---@param keys string[]
---@return string
local function key_text(keys)
  local parts, first = {}, 1
  for i = 1, #keys do
    if i == #keys or not follows(keys[i], keys[i + 1]) then
      parts[#parts + 1] = i > first and (keys[first] .. "-" .. keys[i]) or keys[i]
      first = i + 1
    end
  end
  local text = parts[1] or ""
  for i = 2, #parts do
    local joint = (is_letter(parts[i - 1]) and is_letter(parts[i])) and "" or ","
    text = text .. joint .. parts[i]
  end
  return text
end

---@class (private) games.KeyHintEntry
---@field keys string
---@field label string
---@field width integer display columns of `keys label`

---Entries packed in order into rows no wider than width; an entry wider than width gets a row
---of its own.
---@param entries games.KeyHintEntry[]
---@param width integer|nil nil packs them all into one row
---@return games.KeyHintEntry[][]
local function wrapped(entries, width)
  local rows, row, row_width = {}, {}, 0
  for _, entry in ipairs(entries) do
    local needed = #row > 0 and row_width + #GAP + entry.width or entry.width
    if #row > 0 and width and needed > width then
      rows[#rows + 1], row, needed = row, {}, entry.width
    end
    row[#row + 1], row_width = entry, needed
  end
  rows[#rows + 1] = row
  return rows
end

---@param row games.KeyHintEntry[]
---@return games.Node
local function row_node(row)
  local parts = {}
  for i, entry in ipairs(row) do
    if i > 1 then
      parts[#parts + 1] = compose.text(GAP)
    end
    parts[#parts + 1] = compose.text(entry.keys, Group.KEY)
    parts[#parts + 1] = compose.text(" ")
    parts[#parts + 1] = compose.text(entry.label, Group.LABEL)
  end
  return compose.row(parts)
end

---Rows of `key label` pairs, e.g. `m move  R resign  q close`, wrapped to width. Adjacent hints
---with one label share it, e.g. `1-7 drop`.
---@param hints games.KeyHint[]
---@param width? integer display columns; nil keeps one row however wide
---@return games.Node
return function(hints, width)
  local entries = vim.tbl_map(function(group)
    local keys = key_text(group.keys)
    local entry_width = text_util.width(keys .. " " .. group.label)
    return { keys = keys, label = group.label, width = entry_width }
  end, grouped(hints))
  return compose.col(vim.tbl_map(row_node, wrapped(entries, width)))
end
