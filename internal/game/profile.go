package game

import (
	"nytrpg/internal/protocol"
	"nytrpg/internal/ranked"
)

// Where profiles get a character's recent ranked duels from (ranked.Service)
type RankedHistory interface {
	Recent(charID int) []protocol.RankedMatchInfo
}

// A player's profile as the world knows it, from the rating kept in memory:
// yourself, or someone in view. Recent duels aren't filled in, they come from
// the database (see Profile). ok is false if target isn't someone you can see.
func (w *World) profileOf(c Client, target protocol.EntityID) (prof protocol.Profile, charID int, ok bool) {
	w.Query(func(w *World) {
		p, found := w.players[c]
		if !found {
			return
		}
		t := p
		if target != p.ent.ID {
			if _, inView := p.known[target]; !inView {
				return
			}
			if t = w.playerByEntity(target); t == nil {
				return
			}
		}
		r := t.rating
		prof = protocol.Profile{
			ID: t.ent.ID, Name: t.ent.Name, Char: t.ent.Char, Class: t.ent.Class,
			Elo: r.Elo, Peak: r.Peak, Games: r.Games, Wins: r.Wins, Losses: r.Losses, Draws: r.Draws,
			Recent: []protocol.RankedMatchInfo{},
		}
		if t != p {
			stakes := ranked.Stakes(p.rating, t.rating)
			prof.Stakes = &stakes
		}
		charID, ok = t.charID, true
	})
	return prof, charID, ok
}

// The target's full profile, including recent duels. Call off the world
// goroutine: it waits on the world and on the database.
func (w *World) Profile(c Client, target protocol.EntityID) (protocol.Profile, bool) {
	prof, charID, ok := w.profileOf(c, target)
	if ok && w.History != nil {
		prof.Recent = w.History.Recent(charID)
	}
	return prof, ok
}
