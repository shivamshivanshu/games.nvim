local picker = require("games.ui.picker")

---vim.ui.select has no preview and one key, so an item runs its default action or asks which.
---@type games.FinderAdapter
return {
  available = function()
    return true
  end,
  open = function(source, handlers)
    picker.pick(source.items, {
      prompt = source.title .. ":",
      format_item = source.format,
    }, handlers.confirm)
  end,
}
