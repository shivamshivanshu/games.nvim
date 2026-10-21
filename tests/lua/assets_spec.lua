local assets = require("games.ui.assets")
local helpers = require("helpers")
local registry = require("games.core.registry")

local assert_eq = helpers.assert_eq

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
local art = require("games.ui.art")
local SPEC = { keys = { "x" }, glyph_width = 1 }

---@param edit? fun(pack: table)
---@return table a pack for SPEC, with edit applied
local function pack(edit)
  local fresh = {
    name = "test",
    glyphs = {
      unicode = { x = { { h = 1, w = 1, lines = { "x" } } } },
      ascii = { x = { { h = 1, w = 1, lines = { "x" } } } },
    },
    art = { x = { { h = 2, w = 2, lines = { "▄▄", "▀▀" } } } },
  }
  if edit then
    edit(fresh)
  end
  return fresh
end

---@param lines string[]
---@return fun(pack: table)
local function with_art(lines)
  return function(edited)
    edited.art.x = { { h = 2, w = 2, lines = lines } }
  end
end

---@param cases {[1]: table, [2]: string|nil}[] a pack, and the start of its problem or nil
---@param spec games.AssetSpec
local function assert_problems(cases, spec)
  for _, case in ipairs(cases) do
    local problem = assets.problem(case[1], spec)
    local matches = case[2] == nil and problem == nil
      or (problem ~= nil and case[2] ~= nil and vim.startswith(problem, case[2]))
    assert(matches, vim.inspect({ expected = case[2], actual = problem }))
  end
end

---Art that several games' packs merge lives in assets/shared and is no game's pack.
local SHARED_DIR = "shared"

---@return {game: string, name: string}[]
local function shipped_packs()
  local paths = vim.fn.glob(REPO_ROOT .. "/lua/games/assets/*/*.lua", false, true)
  return vim
    .iter(paths)
    :map(function(path)
      return { game = vim.fn.fnamemodify(path, ":h:t"), name = vim.fn.fnamemodify(path, ":t:r") }
    end)
    :filter(function(shipped)
      return shipped.game ~= SHARED_DIR
    end)
    :totable()
end

return {
  every_shipped_pack_draws_every_key_of_its_game_under_either_ambiwidth = function()
    local packs = shipped_packs()
    assert(#packs > 0, "no shipped packs found")
    for _, shipped in ipairs(packs) do
      local module = assert(registry.game_module(shipped.game), shipped.game)
      assert(module.asset_keys, shipped.game)
      local spec = assets.spec_of(module)
      local loaded = require(("games.assets.%s.%s"):format(shipped.game, shipped.name))
      local problem = assets.problem(loaded, spec)
      assert(not problem, ("%s/%s: %s"):format(shipped.game, shipped.name, problem))
      for _, ambiwidth in ipairs({ "single", "double" }) do
        helpers.with_overrides({ { vim.o, "ambiwidth", ambiwidth } }, function()
          for _, glyphs in ipairs({ "unicode", "ascii" }) do
            local set = assets.art_set(loaded, glyphs)
            for _, key in ipairs(spec.keys) do
              local glyph = art.glyph(set, key, ambiwidth)
              assert_eq(spec.glyph_width, vim.fn.strdisplaywidth(glyph))
            end
            for _, key in ipairs(spec.pictures or {}) do
              local lines = art.picture(set, key, ambiwidth)
              local widths = vim.tbl_map(vim.fn.strdisplaywidth, lines)
              assert_eq(vim.fn["repeat"]({ widths[1] }, #lines), widths)
            end
          end
        end)
      end
    end
  end,

  problem_names_what_a_pack_gets_wrong = function()
    local cases = {
      { pack(), nil },
      {
        pack(function(edited)
          edited.glyphs.ascii.x = {}
        end),
        "glyphs.ascii.x: no glyph",
      },
      {
        pack(function(edited)
          edited.glyphs.unicode.y = edited.glyphs.unicode.x
        end),
        'glyphs.unicode."y": not a key',
      },
      {
        pack(function(edited)
          edited.glyphs.unicode.x = { { h = 2, w = 1, lines = { "x", "x" } } }
        end),
        "glyphs.unicode.x[1]: a glyph is one line",
      },
      {
        pack(function(edited)
          edited.glyphs.unicode.x = { { h = 1, w = 2, lines = { "中" } } }
        end),
        "glyphs.unicode.x[1]: a glyph is the cell width, w = 1",
      },
      {
        pack(function(edited)
          edited.glyphs.ascii.x = { { h = 1, w = 1, lines = { "·" } } }
        end),
        'glyphs.ascii.x[1]: "·" is not printable ascii',
      },
      { pack(with_art({ "▄▄" })), "art.x[1]: needs exactly h = 2 lines" },
      { pack(with_art({ "▄#", "▀▀" })), "art.x[1]: line 1 holds characters" },
      { pack(with_art({ "▄▄▄", "▀▀" })), "art.x[1]: line 1 is not w = 2 wide" },
      {
        pack(function(edited)
          edited.art = "none"
        end),
        "art: must be a table",
      },
    }
    assert_problems(cases, SPEC)
  end,

  problem_checks_pictures_as_whole_entries_of_any_size = function()
    local spec = { keys = { "x" }, glyph_width = 1, pictures = { "scene" } }
    ---@param unicode string[]
    ---@param ascii string[]
    ---@return table
    local function with_scene(unicode, ascii)
      return pack(function(edited)
        edited.glyphs.unicode.scene = { { h = #unicode, w = 3, lines = unicode } }
        edited.glyphs.ascii.scene = { { h = #ascii, w = 3, lines = ascii } }
      end)
    end
    local cases = {
      { with_scene({ "┌─┐", "└─┘" }, { "+-+", "+-+" }), nil },
      { pack(), "glyphs.unicode.scene: no glyph" },
      {
        with_scene({ "┌─┐", "└┘" }, { "+-+", "+-+" }),
        "glyphs.unicode.scene[1]: line 2 is not w = 3",
      },
      {
        with_scene({ "┌─┐", "└─┘" }, { "+-+", "└─┘" }),
        'glyphs.ascii.scene[1]: "└─┘" is not printable',
      },
      {
        with_scene({ "┌─┐" }, { "+-+", "+-" }),
        "glyphs.ascii.scene[1]: line 2 is not w = 3",
      },
    }
    assert_problems(cases, spec)
    assert(
      vim.startswith(
        assets.problem(with_scene({ "┌─┐" }, { "+-+" }), SPEC) or "",
        'glyphs.unicode."scene": not a key'
      ),
      "a picture the spec does not list is unknown"
    )
  end,

  bad_user_packs_fall_back_to_default_with_one_notice_each = function()
    local throwing = setmetatable({}, {
      __index = function()
        error("boom")
      end,
    })
    local chess_spec = { keys = require("games.games.chess").asset_keys, glyph_width = 1 }
    for module, made in pairs({ ["test.wrong_keys_art"] = pack(), ["test.throwing_art"] = throwing }) do
      package.preload[module] = function()
        return made
      end
      local notices = helpers.capture_notifications(function()
        for _ = 1, 2 do
          assert_eq("default", assets.load("chess", module, chess_spec).name)
        end
      end)
      package.preload[module] = nil
      assert_eq(1, #notices)
      assert(notices[1]:find(("%q is unusable"):format(module), 1, true), notices[1])
    end
  end,

  unicode_art_set_adds_art_then_ascii_fallback_and_ascii_has_only_glyphs = function()
    local set = assets.art_set(pack(), "unicode")
    assert_eq(
      { 1, 2, 1 },
      vim.tbl_map(function(entry)
        return entry.h
      end, set.x)
    )
    assert_eq(1, #assets.art_set(pack(), "ascii").x)
  end,
}
