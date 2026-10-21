local colour = require("games.ui.colour")
local helpers = require("helpers")
local highlights = require("games.ui.highlights")
local theme = require("games.ui.theme")

local assert_eq = helpers.assert_eq

local THEME = { palette = "theme", highlights = {} }
local VIVID = { palette = "vivid", highlights = {} }
---@type table<games.HlRole, string[]> attributes a group of the role must leave unset
local FORBIDDEN_BY_ROLE = {
  bg = { "fg", "ctermfg", "reverse" },
  fg = { "bg", "ctermbg", "reverse" },
  badge = {},
}

---@param group string
---@return vim.api.keyset.get_hl_info the effective attributes, without the `default` flag
local function resolved(group)
  local definition = vim.api.nvim_get_hl(0, { name = group, link = false })
  definition.default = nil
  return definition
end

---Blanks every Games* group, as a fresh session has them.
local function clear_groups()
  for group in pairs(highlights.ROLES) do
    vim.api.nvim_set_hl(0, group, {})
  end
end

---Runs fn with 'termguicolors' and 'background' set, restoring both and the groups after.
---@param termguicolors boolean
---@param background "dark"|"light"
---@param fn fun()
local function with_display(termguicolors, background, fn)
  local saved = { termguicolors = vim.o.termguicolors, background = vim.o.background }
  vim.o.termguicolors = termguicolors
  vim.o.background = background
  clear_groups()
  local ok, err = xpcall(fn, debug.traceback)
  vim.o.termguicolors = saved.termguicolors
  vim.o.background = saved.background
  clear_groups()
  theme.setup(THEME)
  if not ok then
    error(err, 0)
  end
end

local T = {}

function T.seat_groups_have_distinct_colours_and_repeat_after_four()
  with_display(true, "dark", function()
    theme.setup(THEME)
    local seen = {}
    for i = 1, 4 do
      local fg = resolved(theme.seat_hl(i)).fg
      assert(fg and not seen[fg], ("seat %d has fg %s"):format(i, vim.inspect(fg)))
      seen[fg] = true
    end
    assert_eq(theme.seat_hl(1), theme.seat_hl(5))
  end)
end

function T.given_is_the_muted_colour_in_bold_under_every_palette_and_display()
  for _, termguicolors in ipairs({ true, false }) do
    for _, palette in ipairs({ THEME, VIVID }) do
      with_display(termguicolors, "dark", function()
        theme.setup(palette)
        local given, muted = resolved("GamesGiven"), resolved("GamesMuted")
        assert_eq({ fg = muted.fg, ctermfg = muted.ctermfg, bold = true, cterm_bold = true }, {
          fg = given.fg,
          ctermfg = given.ctermfg,
          bold = given.bold,
          cterm_bold = given.cterm and given.cterm.bold,
        })
      end)
    end
  end
end

function T.layer_groups_set_only_their_role_attribute_under_every_palette()
  for _, options in ipairs({ THEME, VIVID }) do
    for _, background in ipairs({ "dark", "light" }) do
      with_display(true, background, function()
        theme.setup(options)
        assert_eq(options.palette, theme.palette())
        for group, role in pairs(highlights.ROLES) do
          local definition = resolved(group)
          for _, attr in ipairs(FORBIDDEN_BY_ROLE[role]) do
            assert(
              definition[attr] == nil,
              ("%s %s (%s): %s"):format(options.palette, background, group, vim.inspect(definition))
            )
          end
        end
      end)
    end
  end
end

function T.theme_rederives_on_colorscheme_taking_only_the_role_attribute()
  with_display(true, "dark", function()
    theme.setup(THEME)
    vim.api.nvim_set_hl(0, "Search", { fg = "#ff0000", bg = "#0000ff" })
    vim.api.nvim_set_hl(0, "IncSearch", { fg = "#00ff00", reverse = true })
    vim.api.nvim_exec_autocmds("ColorScheme", {})
    assert_eq({ bg = 0x0000ff }, resolved("GamesLastMove"))
    assert_eq({ bg = 0x00ff00 }, resolved("GamesWin"))
  end)
end

function T.setup_again_keeps_one_colorscheme_autocmd()
  with_display(true, "dark", function()
    theme.setup(THEME)
    theme.setup(VIVID)
    theme.setup(THEME)
    assert_eq(1, #vim.api.nvim_get_autocmds({ group = "games.theme", event = "ColorScheme" }))
  end)
end

function T.vivid_without_termguicolors_falls_back_to_theme_and_warns_once()
  with_display(false, "dark", function()
    package.loaded["games.ui.theme"] = nil
    local fresh_theme = require("games.ui.theme")
    package.loaded["games.ui.theme"] = theme
    local warnings = helpers.capture_notifications(function()
      fresh_theme.setup(VIVID)
      fresh_theme.setup(VIVID)
    end)
    assert_eq("theme", fresh_theme.palette())
    assert_eq(1, #warnings)
    assert_eq(resolved("CursorLine").ctermbg, resolved("GamesSquareLight").ctermbg)
  end)
end

function T.theme_squares_are_named_by_lightness_on_either_background()
  for _, background in ipairs({ "dark", "light" }) do
    with_display(true, background, function()
      vim.cmd.colorscheme("default")
      theme.setup(THEME)
      local light = resolved("GamesSquareLight").bg
      local dark = resolved("GamesSquareDark").bg
      assert(
        light and dark and colour.luminance(light) > colour.luminance(dark),
        ("%s: light %s, dark %s"):format(background, vim.inspect(light), vim.inspect(dark))
      )
    end)
  end
end

function T.inks_contrast_with_the_background_they_sit_on()
  with_display(true, "dark", function()
    theme.setup(VIVID)
    assert_eq({ fg = 0x1b1b1b }, resolved("GamesPieceWhite"))
    assert_eq({ fg = 0x1b1b1b }, resolved("GamesPieceBlack"))
    assert_eq(0x1b1b1b, resolved("GamesSquareLightInk").fg)
    theme.setup({ palette = "vivid", highlights = { GamesLastMove = { bg = "#202020" } } })
    assert_eq(0xf2f2f2, resolved("GamesLastMoveInk").fg)

    theme.setup(THEME)
    vim.api.nvim_set_hl(0, "Search", { bg = "#000000", ctermbg = 231 })
    vim.api.nvim_exec_autocmds("ColorScheme", {})
    assert_eq({ fg = 0xf2f2f2, ctermfg = 234 }, resolved("GamesLastMoveDarkInk"))
  end)
end

function T.a_link_override_holds_across_colorschemes()
  with_display(true, "dark", function()
    helpers.capture_notifications(function()
      theme.setup({ palette = "theme", highlights = { GamesCheck = { link = "Search" } } })
      for _ = 1, 2 do
        vim.cmd.colorscheme("default")
        local search = resolved("Search")
        assert(search.bg, "Search has no bg")
        assert_eq({ bg = search.bg, ctermbg = search.ctermbg }, resolved("GamesCheck"))
      end
    end)
  end)
end

function T.an_option_override_beats_a_colorschemes_definition_but_the_palette_does_not()
  with_display(true, "dark", function()
    vim.api.nvim_set_hl(0, "GamesKey", { fg = "#111111" })
    vim.api.nvim_set_hl(0, "GamesLabel", { fg = "#333333" })
    theme.setup({ palette = "vivid", highlights = { GamesKey = { fg = "#222222" } } })
    assert_eq({ fg = 0x222222 }, resolved("GamesKey"))
    assert_eq({ fg = 0x333333 }, resolved("GamesLabel"))
  end)
end

function T.an_override_is_cut_to_its_role_and_reported_once()
  with_display(true, "dark", function()
    local options = {
      palette = "theme",
      highlights = { GamesLastMove = { bg = "#123456", fg = "#ff0000", reverse = true } },
    }
    local messages = helpers.capture_notifications(function()
      theme.setup(options)
      theme.setup(options)
    end)
    assert_eq({ bg = 0x123456 }, resolved("GamesLastMove"))
    assert_eq(1, #messages)
    assert(messages[1]:find("fg, reverse", 1, true), messages[1])
  end)
end

function T.option_highlights_replace_the_palette_definition()
  with_display(true, "dark", function()
    theme.setup({ palette = "vivid", highlights = { GamesKey = { fg = "#123456" } } })
    assert_eq({ fg = 0x123456 }, resolved("GamesKey"))
  end)
end

function T.reapplying_keeps_a_user_hi_and_replaces_its_own_definitions()
  with_display(true, "dark", function()
    theme.setup(THEME)
    vim.cmd("highlight GamesKey guifg=#abcdef")
    theme.setup(VIVID)
    vim.api.nvim_exec_autocmds("ColorScheme", {})
    assert_eq(0xabcdef, resolved("GamesKey").fg)
    assert_eq(0xf0d9b5, resolved("GamesSquareLight").bg)
  end)
end

return T
