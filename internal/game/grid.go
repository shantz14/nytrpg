package game

import "nytrpg/internal/protocol"

// Cells are about a screen wide. A player sees their own cell and the 8 around
// it, so everything on screen is always known, plus a margin.
const cellSize = 1024

type cellKey struct{ x, y int32 }

func cellOf(p protocol.Vec) cellKey {
	// Floor division, so negative coordinates don't share cell 0
	return cellKey{floorDiv(p.X, cellSize), floorDiv(p.Y, cellSize)}
}

func floorDiv(a, b int32) int32 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// Uniform spatial grid for finding the entities near a point
type grid struct {
	cells map[cellKey]map[protocol.EntityID]*Entity
}

func newGrid() *grid {
	return &grid{cells: make(map[cellKey]map[protocol.EntityID]*Entity)}
}

func (g *grid) insert(e *Entity) {
	e.cell = cellOf(e.Pos)
	cell := g.cells[e.cell]
	if cell == nil {
		cell = make(map[protocol.EntityID]*Entity)
		g.cells[e.cell] = cell
	}
	cell[e.ID] = e
}

func (g *grid) remove(e *Entity) {
	cell := g.cells[e.cell]
	delete(cell, e.ID)
	if len(cell) == 0 {
		delete(g.cells, e.cell)
	}
}

// Call after changing e.Pos
func (g *grid) moved(e *Entity) {
	if cellOf(e.Pos) != e.cell {
		g.remove(e)
		g.insert(e)
	}
}

// Calls fn for every entity in the 3x3 cells around p
func (g *grid) near(p protocol.Vec, fn func(*Entity)) {
	c := cellOf(p)
	for dx := int32(-1); dx <= 1; dx++ {
		for dy := int32(-1); dy <= 1; dy++ {
			for _, e := range g.cells[cellKey{c.x + dx, c.y + dy}] {
				fn(e)
			}
		}
	}
}
