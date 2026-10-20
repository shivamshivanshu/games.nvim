local assets = require("games.ui.assets")
local helpers = require("helpers")
local registry = require("games.core.registry")

local assert_eq = helpers.assert_eq

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
local KEYS = { "x" }

---@param edit? fun(pack: table)
---@return table a pack for KEYS, with edit applied
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

---@return {game: string, name: string}[]
local function shipped_packs()
  local paths = vim.fn.glob(REPO_ROOT .. "/lua/games/assets/*/*.lua", false, true)
  return vim.tbl_map(function(path)
    return { game = vim.fn.fnamemodify(path, ":h:t"), name = vim.fn.fnamemodify(path, ":t:r") }
  end, paths)
end

return {
  every_shipped_pack_draws_its_game_under_either_ambiwidth = function()
    local packs = shipped_packs()
    assert(#packs > 0, "no shipped packs found")
    for _, ambiwidth in ipairs({ "single", "double" }) do
      helpers.with_overrides({ { vim.o, "ambiwidth", ambiwidth } }, function()
        for _, shipped in ipairs(packs) do
          local module = assert(registry.game_module(shipped.game), shipped.game)
          local loaded = require(("games.assets.%s.%s"):format(shipped.game, shipped.name))
          local problem = assets.problem(loaded, assert(module.asset_keys))
          assert(
            not problem,
            ("%s/%s under %s: %s"):format(shipped.game, shipped.name, ambiwidth, problem)
          )
        end
      end)
    end
  end,

  problem_names_what_a_pack_gets_wrong = function()
    local cases = {
      { pack(), nil },
      {
        pack(function(edited)
          edited.glyphs.ascii.x = {}
        end),
        "glyphs.ascii.x: no one-line glyph",
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
    for _, case in ipairs(cases) do
      local problem = assets.problem(case[1], KEYS)
      local matches = case[2] == nil and problem == nil
        or (problem ~= nil and case[2] ~= nil and vim.startswith(problem, case[2]))
      assert(matches, vim.inspect({ expected = case[2], actual = problem }))
    end
  end,

  a_bad_user_pack_falls_back_to_default_with_one_notice = function()
    package.preload["test.bad_chess_art"] = function()
      return pack()
    end
    local chess = require("games.games.chess")
    local notices = {}
    helpers.with_overrides({
      {
        vim,
        "notify",
        function(message)
          notices[#notices + 1] = message
        end,
      },
    }, function()
      for _ = 1, 2 do
        assert_eq("default", assets.load("chess", "test.bad_chess_art", chess.asset_keys).name)
      end
    end)
    package.preload["test.bad_chess_art"] = nil
    assert_eq(1, #notices)
    assert(notices[1]:find('"test.bad_chess_art" is unusable', 1, true), notices[1])
  end,

  unicode_art_set_adds_art_to_glyphs_and_ascii_has_none = function()
    local set = assets.art_set(pack(), "unicode")
    assert_eq(
      { 1, 2 },
      vim.tbl_map(function(entry)
        return entry.h
      end, set.x)
    )
    assert_eq(1, #assets.art_set(pack(), "ascii").x)
  end,
}
