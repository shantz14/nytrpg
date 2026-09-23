package game

import (
	"log/slog"

	"nytrpg/internal/protocol"
)

// Sends each player what changed near them since the last tick: entities that
// came into view (spawn), known ones that moved, and ones that left (despawn).
// Players see nothing outside the cells around them, and get nothing at all when
// nothing near them changed.
func (w *World) replicate() {
	for _, p := range w.players {
		var upd protocol.WorldUpdate

		w.grid.near(p.ent.Pos, func(e *Entity) {
			if e == p.ent {
				return
			}
			if _, known := p.known[e.ID]; !known {
				upd.Spawn = append(upd.Spawn, e.spawnMsg())
			} else if e.moved {
				upd.Move = append(upd.Move, protocol.EntityMove{ID: e.ID, X: e.Pos.X, Y: e.Pos.Y})
			}
			p.known[e.ID] = w.tick
		})

		// Anything known but not seen this tick is out of view or gone
		for id, seen := range p.known {
			if seen != w.tick {
				upd.Despawn = append(upd.Despawn, id)
				delete(p.known, id)
			}
		}

		if len(upd.Spawn) == 0 && len(upd.Move) == 0 && len(upd.Despawn) == 0 {
			continue
		}
		msg, err := protocol.Encode(protocol.ServerWorld, upd)
		if err != nil {
			slog.Error("encoding world update", "err", err)
			continue
		}
		w.Stats.BytesOut.Add(int64(len(msg)))
		p.client.Send(msg)
	}

	for _, e := range w.moved {
		e.moved = false
	}
	w.moved = w.moved[:0]
}
