package fifteen

import (
	"container/heap"
	"slices"
	"testing"
)

// greed weighs the distance estimate over the moves made, trading the shortest solution for a
// quick one.
const greed = 3

// searchLimit fails a search that an unsolvable board would run forever.
const searchLimit = 1_000_000

type searchNode struct {
	b        board
	moves    int
	priority int
	tile     int
	parent   *searchNode
}

type frontier []*searchNode

func (f frontier) Len() int           { return len(f) }
func (f frontier) Less(i, j int) bool { return f[i].priority < f[j].priority }
func (f frontier) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f *frontier) Push(x any)        { *f = append(*f, x.(*searchNode)) }
func (f *frontier) Pop() any {
	old := *f
	last := old[len(old)-1]
	*f = old[:len(old)-1]
	return last
}

// solve returns tiles to slide, in order, that solve b: weighted A* on the Manhattan distance.
func solve(t testing.TB, b board) []int {
	t.Helper()
	open := &frontier{{b: b, priority: greed * manhattan(b)}}
	seen := map[string]bool{}
	for expanded := 0; open.Len() > 0 && expanded < searchLimit; expanded++ {
		node := heap.Pop(open).(*searchNode)
		if node.b.solved() {
			return pathTo(node)
		}
		key := string(tileBytes(node.b.tiles))
		if seen[key] {
			continue
		}
		seen[key] = true
		for _, m := range node.b.movable() {
			next := node.b.slid(m.Tile)
			heap.Push(open, &searchNode{b: next, moves: node.moves + 1, priority: node.moves + 1 + greed*manhattan(next), tile: m.Tile, parent: node})
		}
	}
	t.Fatalf("no solution for %v within %d expansions", b.tiles, searchLimit)
	return nil
}

func pathTo(node *searchNode) []int {
	var tiles []int
	for ; node.parent != nil; node = node.parent {
		tiles = append(tiles, node.tile)
	}
	slices.Reverse(tiles)
	return tiles
}

func tileBytes(tiles []int) []byte {
	out := make([]byte, len(tiles))
	for i, tile := range tiles {
		out[i] = byte(tile)
	}
	return out
}

func manhattan(b board) int {
	distance := 0
	for i, tile := range b.tiles {
		if tile == Blank {
			continue
		}
		at, home := b.point(i), b.point(tile-1)
		distance += abs(at.Row-home.Row) + abs(at.Col-home.Col)
	}
	return distance
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
