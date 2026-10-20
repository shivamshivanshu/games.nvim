---fzf spells keys its own way.
---@type table<string, string> vim key notation -> fzf key name
local FZF_KEYS = { ["<M-CR>"] = "alt-enter" }
local FIELD_SEPARATOR = "\t"

---fzf hands back lines, so each line leads with its item's index, hidden from the list.
---@param source games.Source
---@return string[]
local function numbered_lines(source)
  return vim
    .iter(ipairs(source.items))
    :map(function(index, item)
      return index .. FIELD_SEPARATOR .. source.format(item)
    end)
    :totable()
end

---@param source games.Source
---@param selected string[]|nil
---@return any|nil
local function item_of(source, selected)
  local index = selected
    and selected[1]
    and tonumber(selected[1]:match("^(%d+)" .. FIELD_SEPARATOR))
  return index and source.items[index]
end

---@type games.FinderAdapter
return {
  available = function()
    return (pcall(require, "fzf-lua"))
  end,
  open = function(source, handlers)
    ---@param picked fun(item: any)
    ---@return fun(selected: string[])
    local function on_selected(picked)
      return function(selected)
        local item = item_of(source, selected)
        if item then
          vim.schedule(function()
            picked(item)
          end)
        end
      end
    end
    local choose_key = assert(FZF_KEYS[handlers.choose_key], handlers.choose_key)
    require("fzf-lua").fzf_exec(numbered_lines(source), {
      prompt = source.title .. "> ",
      fzf_opts = { ["--delimiter"] = FIELD_SEPARATOR, ["--with-nth"] = "2.." },
      preview = function(selected)
        local item = item_of(source, selected)
        return table.concat(item and source.preview(item) or {}, "\n")
      end,
      actions = {
        enter = on_selected(handlers.confirm),
        [choose_key] = on_selected(handlers.choose),
      },
    })
  end,
}
