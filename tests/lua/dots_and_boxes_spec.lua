local dots_and_boxes = require("games.games.dots_and_boxes")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local text_util = require("games.ui.text")

local assert_eq = helpers.assert_eq

local CATALOGUED = helpers.catalogued("dots_and_boxes")
local FIRST_BOARD_ROW = 5
local CONTROLS = { "resign", "offer_draw" }

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("dots_and_boxes", dots_and_boxes, glyphs, extra)
end

---@param rows integer
---@param cols integer
---@return string[][] rows of cols, every one ""
local function blank(rows, cols)
  local out = {}
  for r = 1, rows do
    out[r] = vim.fn["repeat"]({ "" }, cols)
  end
  return out
end

---@param n integer boxes a side
---@param drawn table<string, games.DotsAndBoxesSeat> edge name -> who drew it
---@param boxes? table<string, games.DotsAndBoxesSeat> box name, its top left dot's -> owner
---@param to_move? games.DotsAndBoxesSeat|false
---@return games.DotsAndBoxesView every undrawn edge legal for the seat to move
local function view_of(n, drawn, boxes, to_move)
  local letters = "abcdefgh"
  local view = {
    across = blank(n + 1, n),
    down = blank(n, n + 1),
    boxes = blank(n, n),
    scores = { p1 = 0, p2 = 0 },
    legal_edges = {},
  }
  local function each_edge(fn)
    for r = 1, n + 1 do
      for c = 1, n + 1 do
        if c <= n then
          fn(r, c, "across", letters:sub(c, c) .. r .. "h")
        end
        if r <= n then
          fn(r, c, "down", letters:sub(c, c) .. r .. "v")
        end
      end
    end
  end
  each_edge(function(r, c, orientation, name)
    if drawn[name] then
      view[orientation][r][c] = drawn[name]
    elseif to_move then
      view.legal_edges[#view.legal_edges + 1] = { row = r, col = c, orientation = orientation }
    end
  end)
  for name, owner in pairs(boxes or {}) do
    local c, r = letters:find(name:sub(1, 1)), tonumber(name:sub(2))
    view.boxes[r][c] = owner
    view.scores[owner] = view.scores[owner] + 1
  end
  return view
end

---Mid game on 3 boxes, bob (p2) to move: he closed a1 last with b1v; alice drew a1h and a1v.
---@param overrides? table fields over the update
---@return games.DotsAndBoxesUpdate
local function midgame(overrides)
  local view = view_of(3, { a1h = "p1", a2h = "p2", a1v = "p1", b1v = "p2" }, { a1 = "p2" }, "p2")
  view.last = { row = 1, col = 2, orientation = "down" }
  return helpers.game_update("dots_and_boxes", {
    seq = 4,
    options = { size = "3", first = "me", time_control = "none" },
    players = { { user = "alice", seat = "p1" }, { user = "bob", seat = "p2" } },
    to_act = { "bob" },
    controls = CONTROLS,
    draw_agreed = {},
    view = view,
  }, overrides)
end

---@param state games.DotsAndBoxesUpdate
---@param view games.ViewOptions
---@param me? games.UserID default bob
---@param display? games.Display
---@return games.Rendered
---@return games.KeyBinding[]
local function render(state, view, me, display)
  local rendered, _, bindings =
    shell.render(state, dots_and_boxes, view, me or "bob", 0, display, CATALOGUED)
  return rendered, bindings
end

---@param rendered games.Rendered
---@param from integer first line
---@param count integer
---@return string[] those lines without trailing space
local function lines_of(rendered, from, count)
  return vim.tbl_map(function(line)
    return (line:gsub("%s+$", ""))
  end, vim.list_slice(rendered.lines, from, from + count - 1))
end

---@param bindings games.KeyBinding[]
---@param key string
---@return games.KeyBinding|nil
local function binding(bindings, key)
  return vim.iter(bindings):find(function(b)
    return b.key == key
  end)
end

local T = {}

function T.the_board_draws_dots_edges_owned_boxes_the_latest_edge_and_the_cursor_by_glyph()
  local at = { row = 2, col = 2, orientation = "down" }
  for glyphs, want in pairs({
    unicode = {
      "1 ·───·   ·   ·   your move",
      "  │ ○ ┃           at b2v",
      "2 ·───·   ·   ·   last b1v",
      "      ┆",
      "3 ·   ·   ·   ·",
      "",
      "4 ·   ·   ·   ·",
      "  a   b   c   d",
    },
    ascii = {
      "1 +---+   +   +   your move",
      "  | O #           at b2v",
      "2 +---+   +   +   last b1v",
      "      :",
      "3 +   +   +   +",
      "",
      "4 +   +   +   +",
      "  a   b   c   d",
    },
  }) do
    local rendered = render(midgame(), view_options(glyphs, { cursor = at }))
    assert_eq(want, lines_of(rendered, FIRST_BOARD_ROW, 8))
  end
  local rendered = render(midgame(), view_options("ascii", { cursor = at }))
  assert_eq("GamesLastMove", helpers.hl_at(rendered, FIRST_BOARD_ROW, 6))
  assert_eq("GamesMovable", helpers.hl_at(rendered, FIRST_BOARD_ROW + 2, 6))
end

function T.a_box_takes_its_owners_glyph_and_header_colour()
  local rendered = render(midgame(), view_options("ascii"))
  local box = helpers.hl_at(rendered, FIRST_BOARD_ROW, 3)
  assert_eq("GamesSeat2", type(box) == "table" and box[#box] or box)
  assert_eq("O", rendered.lines[FIRST_BOARD_ROW + 1]:sub(5, 5))
end

function T.the_cursor_on_a_drawn_edge_is_blocked_and_enter_sends_nothing()
  local at = { row = 1, col = 1, orientation = "across" }
  local rendered, bindings = render(midgame(), view_options("ascii", { cursor = at }))
  assert_eq(
    { "1 +xxx+   +   +   your move", "  | O #           at a1h: drawn" },
    lines_of(rendered, FIRST_BOARD_ROW, 2)
  )
  assert_eq("GamesCheck", helpers.hl_at(rendered, FIRST_BOARD_ROW - 1, 3))
  assert_eq(nil, binding(bindings, "<CR>"))
end

function T.enter_sends_the_edge_under_the_cursor_lying_its_way()
  local function sent(at)
    local state = midgame()
    local _, bindings = render(state, view_options("ascii", { cursor = at }))
    local acted
    binding(bindings, "<CR>").run({
      update = state,
      client = {
        game_act = function(_, request)
          acted = request.action
        end,
      },
    })
    return acted
  end
  assert_eq({ kind = "draw", edge = "c4h" }, sent({ row = 4, col = 3, orientation = "across" }))
  assert_eq({ kind = "draw", edge = "d3v" }, sent({ row = 3, col = 4, orientation = "down" }))
  assert_eq({ kind = "draw", edge = "b2v" }, sent({ row = 2, col = 2, orientation = "down" }))
end

function T.the_cursor_keeps_to_the_edges_of_the_boards_size()
  local by_key = {}
  for _, action in ipairs(dots_and_boxes.actions) do
    by_key[action.key] = action.local_fn
  end
  local function after(state, keys)
    local view = view_options("ascii")
    for key in keys:gmatch(".") do
      view = by_key[key](view, state)
    end
    return view.cursor
  end
  local five = midgame({ view = view_of(5, {}, {}, "p2") })
  five.view.last = nil
  for _, case in ipairs({
    { midgame(), "llllljjjjj", { row = 4, col = 3, orientation = "across" } },
    { midgame(), "llllljjjjjr", { row = 3, col = 3, orientation = "down" } },
    { midgame(), "rllllljjjjj", { row = 3, col = 4, orientation = "down" } },
    { five, "llllllljjjjjjj", { row = 6, col = 5, orientation = "across" } },
  }) do
    assert_eq({ case[2], case[3] }, { case[2], after(case[1], case[2]) })
  end
end

function T.an_80_column_window_wraps_the_footer_while_abort_is_offered()
  local opening = midgame({ controls = { "resign", "offer_draw", "abort" } })
  local view = view_options("ascii", { cursor = { row = 2, col = 2, orientation = "down" } })
  local rendered = render(opening, view, "bob", { text_area = { w = 80, h = 40 } })
  assert_eq({
    "",
    "R resign  D offer draw  A abort  hjkl move  r turn  <CR> draw  ]g next game",
    "q close",
  }, vim.list_slice(rendered.lines, #rendered.lines - 2))
end

function T.a_wrapped_footer_keeps_the_board_inside_the_window()
  local opening = midgame({ controls = { "resign", "offer_draw", "abort" } })
  local view = view_options("unicode", { cursor = { row = 2, col = 2, orientation = "down" } })
  for h = 16, 50 do
    local rendered = render(opening, view, "bob", { text_area = { w = 80, h = h } })
    assert(#rendered.lines <= h, ("%d lines in a window %d tall"):format(#rendered.lines, h))
  end
end

function T.keys_follow_the_turn()
  local waiting = midgame({ to_act = { "bob" } })
  local over = midgame({
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = { "bob" }, reason = "more_boxes" },
  })
  over.view.legal_edges = {}
  local opening = midgame({ controls = { "resign", "offer_draw", "abort" } })
  for _, case in ipairs({
    {
      midgame(),
      "bob",
      "R resign  D offer draw  hjkl move  r turn  <CR> draw  ]g next game  q close",
    },
    {
      opening,
      "bob",
      "R resign  D offer draw  A abort  hjkl move  r turn  <CR> draw  ]g next game  q close",
    },
    { waiting, "alice", "R resign  D offer draw  ]g next game  q close" },
    { over, "bob", "]g next game  q close" },
  }) do
    local state, me, want = case[1], case[2], case[3]
    if me == "alice" then
      state.view.legal_edges = {}
    end
    local rendered = render(
      state,
      view_options("ascii", { cursor = { row = 2, col = 2, orientation = "down" } }),
      me
    )
    assert_eq(want, rendered.lines[#rendered.lines])
  end
  local rendered = render(over, view_options("ascii"))
  assert_eq({ "1 +---+   +   +   game over" }, lines_of(rendered, FIRST_BOARD_ROW, 1))
end

function T.the_header_gives_each_player_its_glyph_and_boxes()
  local rendered = render(midgame(), view_options("unicode"))
  assert_eq(
    { "alice  p1  ● 0 boxes", "bob    p2  ○ 1 box    ←  your move" },
    lines_of(rendered, 1, 2)
  )
end

function T.the_largest_board_and_panel_fit_eighty_by_twenty_four_at_one_line_a_cell()
  local seven = midgame({ view = view_of(7, {}, {}, "p2") })
  seven.view.last = nil
  seven.view.legal_edges = { { row = 1, col = 1, orientation = "across" } }
  local rendered = render(
    seven,
    view_options("unicode", { cursor = { row = 8, col = 7, orientation = "across" } }),
    "bob",
    {
      text_area = { w = 80, h = 22 },
      seats = 2,
    }
  )
  local body = vim.list_slice(rendered.lines, FIRST_BOARD_ROW, #rendered.lines - 2)
  assert(text_util.max_display_width(body) <= 80, table.concat(body, "\n"))
  assert(#rendered.lines <= 22, #rendered.lines)
  assert_eq(
    "8 ·   ·   ·   ·   ·   ·   ·×××·",
    lines_of(rendered, FIRST_BOARD_ROW + 14, 1)[1]
  )
end

function T.the_player_waiting_reads_their_move()
  local waiting = midgame({ view = { legal_edges = {} } })
  assert_eq(
    { "1 +---+   +   +   their move" },
    lines_of(render(waiting, view_options("ascii"), "alice"), FIRST_BOARD_ROW, 1)
  )
end

function T.events_name_the_drawer_the_edge_the_boxes_it_completed_and_a_move_again()
  local function drawn(by, boxes, again)
    return {
      kind = "drawn",
      by = by,
      edge = { row = 1, col = 2, orientation = "down" },
      boxes = boxes,
      again = again,
    }
  end
  local one, two = { { row = 1, col = 1 } }, { { row = 1, col = 1 }, { row = 1, col = 2 } }
  for _, case in ipairs({
    { drawn("p1", {}, false), "alice drew b1v" },
    { drawn("p2", one, true), "bob drew b1v, completing 1 box, and moves again" },
    { drawn("p2", two, false), "bob drew b1v, completing 2 boxes" },
  }) do
    assert_eq(case[2], helpers.described(midgame(), case[1]))
  end
end

return T
