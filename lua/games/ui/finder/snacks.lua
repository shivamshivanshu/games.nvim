local CHOOSE_ACTION = "games_choose"

---@param source games.Source
---@param item any
---@return string
local function preview_text(source, item)
  return table.concat(source.preview(item) or {}, "\n")
end

---@param picked fun(item: any)
---@return fun(picker: table, entry?: table)
local function closing(picked)
  return function(snacks_picker, entry)
    snacks_picker:close()
    if entry then
      vim.schedule(function()
        picked(entry.games_item)
      end)
    end
  end
end

---@type games.FinderAdapter
return {
  available = function()
    local ok, snacks = pcall(require, "snacks")
    return ok and snacks.picker ~= nil
  end,
  open = function(source, handlers)
    local choose_key = { CHOOSE_ACTION, mode = { "n", "i" } }
    require("snacks").picker.pick({
      title = source.title,
      items = vim
        .iter(ipairs(source.items))
        :map(function(index, item)
          return {
            idx = index,
            text = source.format(item),
            games_item = item,
            preview = { text = preview_text(source, item) },
          }
        end)
        :totable(),
      format = "text",
      preview = "preview",
      confirm = closing(handlers.confirm),
      actions = { [CHOOSE_ACTION] = closing(handlers.choose) },
      win = {
        input = { keys = { [handlers.choose_key] = choose_key } },
        list = { keys = { [handlers.choose_key] = choose_key } },
      },
    })
  end,
}
