package game

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"nytrpg/internal/classes"
	"nytrpg/internal/oracle"
	"nytrpg/internal/protocol"
)

const (
	// How long the gods get to answer a prayer
	PrayerTimeout = 30 * time.Second
	// Prayers waiting on the gods at once, across the server. More are
	// refunded.
	MaxPrayers = 4
)

// What the gods proclaim when they intervene
var divineBanners = map[protocol.DivineFate]string{
	protocol.FateCleanSlate:  "THE GODS HAVE WIPED THE SLATE CLEAN",
	protocol.FateNewWord:     "THE GODS HAVE CHANGED THE WORD",
	protocol.FateSuddenDeath: "THE GODS DEMAND AN END: TWO MINUTES REMAIN",
	protocol.FateRevelation:  "THE GODS REVEAL A LETTER TO ALL",
	protocol.FateFortune:     "THE GODS HAVE SWAPPED YOUR FORTUNES",
}

// The Cleric's abilities: prayers, healing and the gods' whims
func (w *World) castCleric(d *duel, me, them *duelSide, id string, req protocol.DuelCastReq) {
	cost := abilityCost(classes.ID(me.p.ent.Class), id)
	switch id {
	case classes.MinorPrayer:
		w.pray(d, me, id, protocol.PrayerMinor, cost)
	case classes.MajorPrayer:
		w.pray(d, me, id, protocol.PrayerMajor, cost)
	case classes.Mend:
		w.mend(d, me, them, req.Row)
	case classes.Purify:
		w.purify(d, me, them)
	case classes.DivineIntervention:
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
		w.intervene(d)
	}
}

// Takes back one of me's guesses: later rows move up, and they get the row
// back to guess in
func (w *World) mend(d *duel, me, them *duelSide, row int) {
	me.guesses = slices.Delete(me.guesses, row, row+1)
	destroyed := make(map[tile]bool, len(me.destroyed))
	for t := range me.destroyed {
		if t.row == row {
			continue
		}
		if t.row > row {
			t.row--
		}
		destroyed[t] = true
	}
	me.destroyed = destroyed

	// Seeing Eyes on me follow their letter up, or look elsewhere if it went
	moved := false
	for _, e := range them.eyes {
		switch {
		case !e.on:
		case e.at.row == row:
			w.aimEye(them, me, e)
			moved = true
		case e.at.row > row:
			e.at.row--
			moved = true
		}
	}
	if moved {
		w.sendEyes(them, me)
	}

	w.castEvent(d, me, classes.Mend, protocol.CastUsed, false, tile{row: row})
	w.send(me.p.client, protocol.ServerDuelGuessRemoved, protocol.DuelGuessRemoved{Yours: true, Row: row})
	w.send(them.p.client, protocol.ServerDuelGuessRemoved, protocol.DuelGuessRemoved{Row: row})
}

// Clears every curse on me. What's done stays done: destroyed letters and
// lost rows.
func (w *World) purify(d *duel, me, them *duelSide) {
	w.castEvent(d, me, classes.Purify, protocol.CastUsed, false, tile{})
	me.stunnedUntil, me.silencedUntil = time.Time{}, time.Time{}
	me.keymap, me.scrambledGuesses = nil, 0
	me.falseNext = nil
	me.missiles = nil
	if me.illusion != nil {
		w.send(me.p.client, protocol.ServerIllusionEnd, protocol.IllusionEnd{Purified: true, Solution: me.illusion.word})
		me.illusion = nil
	}
	if len(them.eyes) > 0 {
		them.eyes = nil
		w.sendEyes(them, me)
	}
}

// Sends a prayer to the gods. They answer later, off the world goroutine: the
// answer comes back through w.Do. If none does, the energy is given back.
func (w *World) pray(d *duel, me *duelSide, ability string, kind protocol.PrayerKind, cost int) {
	w.nextPrayer++
	id := w.nextPrayer
	w.castEvent(d, me, ability, protocol.CastUsed, false, tile{})
	w.send(me.p.client, protocol.ServerDuelPrayer, protocol.DuelPrayer{ID: id, Kind: kind, Pending: true})

	p := oracle.Prayer{Kind: kind, Word: me.word}
	if kind == protocol.PrayerMinor {
		p.Pos = w.prayerLetter(me)
	}
	o := w.Oracle
	if o == nil {
		w.unanswered(me, id, kind, cost)
		return
	}
	select {
	case w.prayerSlots <- struct{}{}:
	default:
		w.unanswered(me, id, kind, cost)
		return
	}
	go func() {
		defer func() { <-w.prayerSlots }()
		ctx, cancel := context.WithTimeout(context.Background(), PrayerTimeout)
		defer cancel()
		ans, err := o.Pray(ctx, p)
		w.Do(func(w *World) {
			switch {
			case me.p.duel != d:
				// The duel's over, nobody's listening
			case err != nil:
				slog.Warn("prayer unanswered", "err", err)
				w.unanswered(me, id, kind, cost)
				w.syncDuel(d)
			case me.word != p.Word:
				// Reality shifted while they prayed, the answer's about a
				// word that's gone
				w.unanswered(me, id, kind, cost)
				w.syncDuel(d)
			default:
				w.send(me.p.client, protocol.ServerDuelPrayer, protocol.DuelPrayer{
					ID: id, Kind: kind, God: ans.God.Name, GodTitle: ans.God.Title, GodColor: ans.God.Color, Text: ans.Text,
				})
			}
		})
	}()
}

// No god answered: the energy comes back
func (w *World) unanswered(me *duelSide, id int, kind protocol.PrayerKind, cost int) {
	me.energy += cost
	w.send(me.p.client, protocol.ServerDuelPrayer, protocol.DuelPrayer{ID: id, Kind: kind, Failed: true, Refunded: cost})
}

// A letter of their word to pray about: one they haven't found in place yet
func (w *World) prayerLetter(s *duelSide) int {
	var open []int
	for i, found := range s.greens {
		if !found {
			open = append(open, i)
		}
	}
	if len(open) == 0 {
		return w.rng.Intn(len(s.word))
	}
	return open[w.rng.Intn(len(open))]
}

// Divine Intervention: the gods pick a fate for the duel and proclaim it
func (w *World) intervene(d *duel) {
	fate := protocol.DivineFate(w.rng.Intn(len(divineBanners)))
	if w.fate != nil {
		fate = w.fate()
	}
	for _, s := range d.sides {
		w.send(s.p.client, protocol.ServerDuelDivine, protocol.DuelDivine{Fate: fate, Banner: divineBanners[fate]})
	}
	now := w.now()
	a, b := d.sides[0], d.sides[1]
	switch fate {
	case protocol.FateCleanSlate:
		for _, s := range d.sides {
			if s.illusion != nil {
				w.send(s.p.client, protocol.ServerIllusionEnd, protocol.IllusionEnd{Purified: true, Solution: s.illusion.word})
			}
			s.guesses = nil
			s.rows = w.Duels.MaxGuesses()
			s.best = 0
			s.sideEffects = newSideEffects(len(s.word))
		}
		d.deadline = time.Time{}
		d.nextEnergy = now.Add(EnergyInterval)
	case protocol.FateNewWord:
		word := w.divineWord(d)
		for _, s := range d.sides {
			w.rescore(s, word)
		}
		for _, s := range d.sides {
			_, other := d.sidesOf(s.p)
			colors := s.visibleColors()
			w.send(s.p.client, protocol.ServerDuelBoard, protocol.DuelBoard{Yours: true, Colors: colors})
			w.send(other.p.client, protocol.ServerDuelBoard, protocol.DuelBoard{Colors: colors})
		}
		w.reaimEyes(a, b)
		w.reaimEyes(b, a)
	case protocol.FateSuddenDeath:
		if until := now.Add(SuddenDeath); d.deadline.IsZero() || until.Before(d.deadline) {
			d.deadline = until
		}
	case protocol.FateRevelation:
		var open []int
		for i := range a.greens {
			if !a.greens[i] && !b.greens[i] {
				open = append(open, i)
			}
		}
		col := w.rng.Intn(len(a.word))
		if len(open) > 0 {
			col = open[w.rng.Intn(len(open))]
		}
		for _, s := range d.sides {
			w.send(s.p.client, protocol.ServerDuelReveal, protocol.DuelReveal{Col: col, Letter: string(s.word[col])})
		}
	case protocol.FateFortune:
		a.energy, b.energy = b.energy, a.energy
	}
}

// A new word for both sides: neither's word, and nothing either has guessed,
// so it never solves itself
func (w *World) divineWord(d *duel) string {
	taken := map[string]bool{}
	for _, s := range d.sides {
		taken[s.word] = true
		for _, g := range s.guesses {
			taken[g.word] = true
		}
	}
	word := d.sides[0].word
	for range 100 {
		if next := w.Duels.NewWord(w.rng); !taken[next] {
			return next
		}
	}
	// Every word tried is taken (a fixed test word): leave it
	return word
}
