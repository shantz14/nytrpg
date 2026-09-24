package game

import (
	"testing"

	"nytrpg/internal/protocol"
)

func TestCellOfNegativeCoordinates(t *testing.T) {
	// Floor division: -1 is in cell -1, not cell 0
	cases := map[int32]int32{0: 0, cellSize - 1: 0, cellSize: 1, -1: -1, -cellSize: -1, -cellSize - 1: -2}
	for x, want := range cases {
		if got := cellOf(protocol.Vec{X: x}).x; got != want {
			t.Errorf("cellOf(%d) = %d, want %d", x, got, want)
		}
	}
}

func TestGridNearAndMove(t *testing.T) {
	g := newGrid()
	a := &Entity{ID: 1, Pos: protocol.Vec{X: 10, Y: 10}}
	b := &Entity{ID: 2, Pos: protocol.Vec{X: cellSize + 10, Y: 10}}   // next cell: near
	c := &Entity{ID: 3, Pos: protocol.Vec{X: 3*cellSize + 10, Y: 10}} // 3 cells away: not near
	for _, e := range []*Entity{a, b, c} {
		g.insert(e)
	}
	near := func() map[protocol.EntityID]bool {
		out := map[protocol.EntityID]bool{}
		g.near(a.Pos, func(e *Entity) { out[e.ID] = true })
		return out
	}
	if n := near(); !n[1] || !n[2] || n[3] {
		t.Fatalf("near a: %v", n)
	}

	// Moving into the neighbouring cell brings c into view
	c.Pos.X = cellSize + 500
	g.moved(c)
	if n := near(); !n[3] {
		t.Fatal("c moved next door but isn't near")
	}
}

func TestGridRemoveCleansUpCells(t *testing.T) {
	g := newGrid()
	e := &Entity{ID: 1}
	g.insert(e)
	g.remove(e)
	if len(g.cells) != 0 {
		t.Fatalf("empty cells should be deleted, have %d", len(g.cells))
	}
}

func TestTownMapLoads(t *testing.T) {
	m, err := LoadMap("town")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findInteractable(m, "wordle"); !ok {
		t.Fatal("town has no wordle board")
	}
	// Players must spawn close enough to the wordle board to use it
	it, _ := findInteractable(m, "wordle")
	if d := distToInteractable(m.Spawn, it); d+spawnSpread*1.5 > float64(it.Range) {
		t.Fatalf("spawn is %.0fpx from the board, range is %d", d, it.Range)
	}
	if it.Label != "Daily Wordle" {
		t.Fatalf("wordle board should be signed Daily Wordle, got %q", it.Label)
	}
	if _, err := LoadMap("nope"); err == nil {
		t.Fatal("missing map loaded")
	}
}

func TestDistToInteractable(t *testing.T) {
	it := protocol.Interactable{Pos: protocol.Vec{X: 100, Y: 100}, W: 50, H: 50}
	cases := []struct {
		p    protocol.Vec
		want float64
	}{
		{protocol.Vec{X: 120, Y: 120}, 0},  // inside
		{protocol.Vec{X: 90, Y: 120}, 10},  // left
		{protocol.Vec{X: 160, Y: 120}, 10}, // right edge is x+w
		{protocol.Vec{X: 153, Y: 154}, 5},  // corner: 3,4,5
	}
	for _, c := range cases {
		if got := distToInteractable(c.p, it); got != c.want {
			t.Errorf("dist(%+v) = %v, want %v", c.p, got, c.want)
		}
	}
}
