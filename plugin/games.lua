if vim.g.loaded_games then
  return
end
vim.g.loaded_games = true

vim.api.nvim_create_user_command("Games", function(command)
  require("games.core.commands").run(command.fargs)
end, {
  nargs = "*",
  desc = "games.nvim: challenge, list and play games",
  complete = function(arg_lead, cmd_line, cursor_pos)
    return require("games.core.commands").complete(arg_lead, cmd_line, cursor_pos)
  end,
})
