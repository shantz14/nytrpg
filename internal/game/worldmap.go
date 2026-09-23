package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"

	"nytrpg/internal/protocol"
)

//go:embed maps/*.json
var mapFiles embed.FS

// Loads a map from internal/game/maps, e.g. LoadMap("town")
func LoadMap(name string) (*protocol.WorldMap, error) {
	data, err := mapFiles.ReadFile("maps/" + name + ".json")
	if err != nil {
		return nil, err
	}
	var m protocol.WorldMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("map %s: %w", name, err)
	}
	if m.Width <= 0 || m.Height <= 0 {
		return nil, fmt.Errorf("map %s: needs a width and height", name)
	}
	if !inBounds(&m, m.Spawn) {
		return nil, fmt.Errorf("map %s: spawn is outside the map", name)
	}
	return &m, nil
}

func inBounds(m *protocol.WorldMap, p protocol.Vec) bool {
	return p.X >= 0 && p.Y >= 0 && p.X <= m.Width && p.Y <= m.Height
}

func findInteractable(m *protocol.WorldMap, id string) (protocol.Interactable, bool) {
	for _, it := range m.Interactables {
		if it.ID == id {
			return it, true
		}
	}
	return protocol.Interactable{}, false
}

// Distance from p to the nearest point of the interactable's rectangle
func distToInteractable(p protocol.Vec, it protocol.Interactable) float64 {
	dx := max(it.Pos.X-p.X, 0, p.X-(it.Pos.X+it.W))
	dy := max(it.Pos.Y-p.Y, 0, p.Y-(it.Pos.Y+it.H))
	return math.Hypot(float64(dx), float64(dy))
}

// True if the client's player is within range of the interactable
func (w *World) InRange(c Client, interactableID string) bool {
	it, ok := findInteractable(w.Map, interactableID)
	if !ok {
		return false
	}
	if it.Range == 0 {
		return true
	}
	near := false
	w.Query(func(w *World) {
		if p, ok := w.players[c]; ok {
			near = distToInteractable(p.ent.Pos, it) <= float64(it.Range)
		}
	})
	return near
}
