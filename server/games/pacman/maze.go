package pacman

import (
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Rows and Cols are the maze's size.
const (
	Rows = 31
	Cols = 28
)

// classicMaze is the arcade's maze, one line per row in the tiles' own letters.
// A line shorter than Cols is open to its end.
const classicMaze = `
############################
#............##............#
#.####.#####.##.#####.####.#
#o####.#####.##.#####.####o#
#.####.#####.##.#####.####.#
#..........................#
#.####.##.########.##.####.#
#.####.##.########.##.####.#
#......##....##....##......#
######.##### ## #####.######
     #.##### ## #####.#
     #.##          ##.#
     #.## ###--### ##.#
######.## #      # ##.######
      .   #      #   .
######.## #      # ##.######
     #.## ######## ##.#
     #.##          ##.#
     #.## ######## ##.#
######.## ######## ##.######
#............##............#
#.####.#####.##.#####.####.#
#.####.#####.##.#####.####.#
#o..##.......  .......##..o#
###.##.##.########.##.##.###
###.##.##.########.##.##.###
#......##....##....##......#
#.##########.##.##########.#
#.##########.##.##########.#
#..........................#
############################`

// Tile is what a maze cell holds, as its letter in View.Maze.
type Tile byte

// The tiles.
const (
	TileOpen   Tile = ' '
	TileWall   Tile = '#'
	TileDoor   Tile = '-'
	TileDot    Tile = '.'
	TilePellet Tile = 'o'
)

// maze is maze[row][col], row 0 at the top. States share one until it changes: eaten
// returns a new maze.
type maze [Rows][Cols]Tile

func classic() *maze {
	var m maze
	for r, line := range strings.Split(strings.TrimPrefix(classicMaze, "\n"), "\n") {
		for c := range Cols {
			m[r][c] = TileOpen
			if c < len(line) {
				m[r][c] = Tile(line[c])
			}
		}
	}
	return &m
}

// wrapped is p with its column brought back onto the maze, so the tunnel's ends meet.
func wrapped(p grid.Point) grid.Point {
	p.Col = (p.Col%Cols + Cols) % Cols
	return p
}

func (m *maze) at(p grid.Point) Tile {
	if p.Row < 0 || p.Row >= Rows {
		return TileWall
	}
	p = wrapped(p)
	return m[p.Row][p.Col]
}

// walkable is whether Pac-Man or a ghost in the maze may stand on p: the door is the ghosts' own.
func (m *maze) walkable(p grid.Point) bool {
	t := m.at(p)
	return t != TileWall && t != TileDoor
}

func (m *maze) eaten(p grid.Point) *maze {
	next := *m
	next[p.Row][p.Col] = TileOpen
	return &next
}

func (m *maze) food() int {
	n := 0
	for _, row := range m {
		for _, t := range row {
			if t == TileDot || t == TilePellet {
				n++
			}
		}
	}
	return n
}

// lines are the maze's rows, cut from one string so the maze is copied once.
func (m *maze) lines() []string {
	var tiles [Rows * Cols]byte
	for r, row := range m {
		for c, t := range row {
			tiles[r*Cols+c] = byte(t)
		}
	}
	all := string(tiles[:])
	lines := make([]string, Rows)
	for r := range lines {
		lines[r] = all[r*Cols : (r+1)*Cols]
	}
	return lines
}

// inTunnel is whether p is in the side tunnel, where ghosts slow down.
func inTunnel(p grid.Point) bool {
	const tunnelRow, tunnelDepth = 14, 6
	return p.Row == tunnelRow && (p.Col < tunnelDepth || p.Col >= Cols-tunnelDepth)
}

// pacmanStart is where Pac-Man starts each life, facing left.
func pacmanStart() grid.Point { return grid.Point{Row: 23, Col: 13} }

// houseExit is the cell above the door: ghosts leave the house through it, and eyes come back to it.
func houseExit() grid.Point { return grid.Point{Row: 11, Col: 13} }

// houseCentre is the cell inside the house that eyes go down to before they leave again.
func houseCentre() grid.Point { return grid.Point{Row: 14, Col: 13} }
