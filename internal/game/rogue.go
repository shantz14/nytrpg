package game

import (
	"slices"

	"nytrpg/internal/classes"
	"nytrpg/internal/protocol"
)

// The Rogue's abilities: tricks with letters, colors and keyboards. Returns
// false if id isn't one.
func (w *World) castRogue(d *duel, me, them *duelSide, id string, req protocol.DuelCastReq) bool {
	switch id {
	case classes.Pickpocket:
		t := tile{req.Row, req.Col}
		if w.hostile(d, me, them, id, protocol.CastUsed, t) {
			w.send(me.p.client, protocol.ServerDuelPickpocket, protocol.DuelPickpocket{
				Row: t.row, Col: t.col, Letter: string(them.guesses[t.row].word[t.col]),
			})
		}
	case classes.Cheat:
		me.cheat = true
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
	case classes.Feint:
		// They know a feint is coming, not what it'll look like
		me.feint = slices.Clone(req.Colors)
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
	case classes.UnderTheirNose:
		// Their shield catches it in plain sight. Otherwise only the caster
		// knows.
		if them.shield {
			w.hostile(d, me, them, id, protocol.CastUsed, tile{})
			break
		}
		them.falseNext = slices.Clone(req.Colors)
		w.send(me.p.client, protocol.ServerDuelCast, protocol.DuelCast{ByYou: true, Ability: id})
	case classes.Confuse:
		if w.hostile(d, me, them, id, protocol.CastUsed, tile{}) {
			them.keymap = w.scrambledKeys()
			them.scrambledGuesses = ScrambleGuesses
		}
	default:
		return false
	}
	return true
}

// Every letter on the keyboard somewhere else
func (w *World) scrambledKeys() []byte {
	keys := make([]byte, 26)
	for i, j := range w.rng.Perm(26) {
		keys[i] = byte('A' + j)
	}
	return keys
}

// Sneaky: two keys on their keyboard swap, on top of any scramble, for their
// next ScrambleGuesses guesses
func (w *World) swapKeys(s *duelSide) {
	if s.keymap == nil {
		s.keymap = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	}
	i := w.rng.Intn(26)
	j := (i + 1 + w.rng.Intn(25)) % 26
	s.keymap[i], s.keymap[j] = s.keymap[j], s.keymap[i]
	s.scrambledGuesses = ScrambleGuesses
}
