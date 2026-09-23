package game

import (
	"math"
	"time"

	"nytrpg/internal/protocol"
)

const (
	// Fastest a player may move, px/s. The client moves at exactly this speed.
	MoveSpeed = 450.0
	// Slack for timer and rounding differences between client and server
	moveTolerance = 1.25
	// Seconds of movement a player can save up. Moves arrive in bursts when the
	// network hiccups, this absorbs that without letting anyone teleport far.
	maxMoveBank = 1.0
)

// Movement allowance. It fills at the max speed over time and each move spends
// the distance moved, so bursts are fine but sustained speeding isn't.
type moveBudget struct {
	budget   float64
	budgetAt time.Time
}

func (b *moveBudget) reset(now time.Time) {
	b.budget = MoveSpeed * moveTolerance * maxMoveBank / 2
	b.budgetAt = now
}

func (b *moveBudget) spend(dist float64, now time.Time) bool {
	rate := MoveSpeed * moveTolerance
	b.budget = math.Min(rate*maxMoveBank, b.budget+rate*now.Sub(b.budgetAt).Seconds())
	b.budgetAt = now
	if dist > b.budget {
		return false
	}
	b.budget -= dist
	return true
}

// Moves the client's player to pos if that's possible from where they are,
// otherwise tells the client where they really are.
func (w *World) Move(c Client, pos protocol.Vec) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok || pos == p.ent.Pos {
			return
		}
		dist := math.Hypot(float64(pos.X-p.ent.Pos.X), float64(pos.Y-p.ent.Pos.Y))
		if !inBounds(w.Map, pos) || !p.spend(dist, w.now()) {
			w.Stats.RejectedMoves.Add(1)
			w.send(c, protocol.ServerCorrection, p.ent.Pos)
			return
		}
		p.ent.Pos = pos
		w.entityMoved(p.ent)
	})
}
