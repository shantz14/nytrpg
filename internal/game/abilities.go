package game

import (
	"slices"
	"strings"
	"time"

	"nytrpg/internal/classes"
	"nytrpg/internal/protocol"
	"nytrpg/internal/ranked"
)

// Duel energy and class abilities. Duelists earn energy over time and by
// finding letters, and spend it on their class's abilities (see the classes
// package). Everything here runs on the world goroutine.

const (
	// Both duelists get 1 energy this often
	EnergyInterval = 30 * time.Second
	// Energy for finding a letter for the first time: a new green position,
	// or a letter in the word you hadn't found yet
	GreenEnergy  = 2
	YellowEnergy = 1

	PommelStun     = 10 * time.Second
	AggressiveStun = 5 * time.Second
	// How long a Magic Missile flies, and what it takes if it lands
	MissileFlight = 15 * time.Second
	MissileEnergy = 5
	// A Seeing Eye moves to another letter this often
	EyeInterval = 30 * time.Second
	// An Illusion's guesses, and the energy lost for failing it
	IllusionGuesses = 6
	IllusionEnergy  = 5
	// Words offered by Reshape Reality
	ReshapeOptions = 5
)

// A letter in a board, by guess and position
type tile struct{ row, col int }

// A Seeing Eye, watching one letter in the opponent's guesses
type eye struct {
	at tile
	// Whether it's on a letter, there may be none it can see
	on   bool
	next time.Time
}

// A small Wordle an Illusion traps someone in
type illusionGame struct {
	word    string
	guesses int
}

// Energy, and what abilities have done to one side of a duel
type sideEffects struct {
	energy int
	// Green positions and letters they've found, so letters only pay the
	// first time
	greens []bool
	found  [26]bool
	// Their keyboard doesn't work until then
	stunnedUntil time.Time
	shield       bool
	// Letters in their guesses destroyed by Slash
	destroyed map[tile]bool
	// Their Seeing Eyes, on the opponent's guesses
	eyes []*eye
	// Magic Missiles flying at them, when each lands
	missiles []time.Time
	// The Illusion they're trapped in, nil when none
	illusion *illusionGame
	// Words they were offered by Reshape Reality, waiting for a pick
	reshape []string
	// Once-only abilities they used
	used map[string]bool
}

func newSideEffects(wordLength int) sideEffects {
	return sideEffects{greens: make([]bool, wordLength), destroyed: make(map[tile]bool), used: make(map[string]bool)}
}

// Can't guess or type in the duel: stunned, or in an Illusion
func (s *duelSide) blocked(now time.Time) bool {
	return now.Before(s.stunnedUntil) || s.illusion != nil
}

func (s *duelSide) stun(until time.Time) {
	if until.After(s.stunnedUntil) {
		s.stunnedUntil = until
	}
}

func (s *duelSide) loseEnergy(n int) int {
	n = min(n, s.energy)
	s.energy -= n
	return n
}

// Records the letters a guess found. Letters found for the first time pay
// energy if pay is set. Returns whether it found a new green.
func (s *duelSide) learn(g guessRow, pay bool) (newGreen bool) {
	gain := 0
	for i, c := range g.colors {
		if c == protocol.Green && i < len(s.greens) && !s.greens[i] {
			s.greens[i] = true
			newGreen = true
			gain += GreenEnergy
			s.markFound(g.word[i])
		}
	}
	for i, c := range g.colors {
		if c == protocol.Yellow && s.markFound(g.word[i]) {
			gain += YellowEnergy
		}
	}
	if pay {
		s.energy += gain
	}
	return newGreen
}

// Returns whether the letter is new
func (s *duelSide) markFound(letter byte) bool {
	i := int(letter) - 'A'
	if i < 0 || i >= len(s.found) || s.found[i] {
		return false
	}
	s.found[i] = true
	return true
}

// Forgets what they found and learns it again from their guesses, without
// paying, after their word changed
func (s *duelSide) relearn() {
	clear(s.greens)
	s.found = [26]bool{}
	s.best = 0
	for _, g := range s.guesses {
		s.learn(g, false)
		s.best = max(s.best, ranked.Closeness(g.colors))
	}
}

// Their guesses' colors, with destroyed letters Hidden
func (s *duelSide) visibleColors() [][]protocol.WordleColor {
	out := make([][]protocol.WordleColor, len(s.guesses))
	for r, g := range s.guesses {
		out[r] = slices.Clone(g.colors)
		for c := range out[r] {
			if s.destroyed[tile{r, c}] {
				out[r][c] = protocol.Hidden
			}
		}
	}
	return out
}

func (s *duelSide) state(now time.Time) protocol.DuelSideState {
	st := protocol.DuelSideState{
		Energy:     s.energy,
		Rows:       s.rows,
		Guesses:    len(s.guesses),
		Shield:     s.shield,
		Eyes:       len(s.eyes),
		Illusion:   s.illusion != nil,
		MissilesMs: []int{},
		Used:       []string{},
	}
	if now.Before(s.stunnedUntil) {
		st.StunnedMs = int(s.stunnedUntil.Sub(now) / time.Millisecond)
	}
	for _, at := range s.missiles {
		st.MissilesMs = append(st.MissilesMs, int(at.Sub(now)/time.Millisecond))
	}
	slices.Sort(st.MissilesMs)
	for id := range s.used {
		st.Used = append(st.Used, id)
	}
	slices.Sort(st.Used)
	return st
}

// Sends both duelists the energy and effects on each side
func (w *World) syncDuel(d *duel) {
	for _, s := range d.sides {
		w.sendDuelState(d, s)
	}
}

func (w *World) sendDuelState(d *duel, s *duelSide) {
	_, other := d.sidesOf(s.p)
	now := w.now()
	w.send(s.p.client, protocol.ServerDuelState, protocol.DuelState{You: s.state(now), Them: other.state(now)})
}

// Tells both duelists something happened with an ability by
func (w *World) castEvent(d *duel, by *duelSide, ability string, kind protocol.DuelCastKind, blocked bool, t tile) {
	for _, s := range d.sides {
		w.send(s.p.client, protocol.ServerDuelCast, protocol.DuelCast{
			ByYou: s == by, Ability: ability, Kind: kind, Blocked: blocked, Row: t.row, Col: t.col,
		})
	}
}

// An ability from caster hits target, unless target's shield stops it (and is
// used up). Tells both players. Returns whether it hit.
func (w *World) hostile(d *duel, caster, target *duelSide, ability string, kind protocol.DuelCastKind, t tile) bool {
	blocked := target.shield
	target.shield = false
	w.castEvent(d, caster, ability, kind, blocked, t)
	return !blocked
}

// After a valid guess that didn't end the duel: pays energy for new letters,
// beats missiles flying at the guesser, sets off passives, and lets idle
// Seeing Eyes look at the new guess
func (w *World) afterGuess(d *duel, me, them *duelSide) {
	now := w.now()
	newGreen := me.learn(me.guesses[len(me.guesses)-1], true)

	for range me.missiles {
		w.castEvent(d, them, classes.MagicMissile, protocol.CastFizzled, false, tile{})
	}
	me.missiles = nil

	if newGreen {
		switch classes.PassiveOf(classes.ID(me.p.ent.Class)) {
		case classes.Aggressive:
			if w.hostile(d, me, them, classes.Aggressive, protocol.CastTriggered, tile{}) {
				them.stun(now.Add(AggressiveStun))
			}
		case classes.Wise:
			if len(me.eyes) > 0 {
				for _, e := range me.eyes {
					w.aimEye(me, them, e)
				}
				w.sendEyes(me, them)
				// Only the wizard needs to know their eyes moved
				w.send(me.p.client, protocol.ServerDuelCast, protocol.DuelCast{ByYou: true, Ability: classes.Wise, Kind: protocol.CastTriggered})
			}
		}
	}

	idle := false
	for _, e := range them.eyes {
		if !e.on {
			w.aimEye(them, me, e)
			idle = idle || e.on
		}
	}
	if idle {
		w.sendEyes(them, me)
	}
	w.syncDuel(d)
}

// Uses the ability in the given slot of the client's class
func (w *World) CastAbility(c Client, req protocol.DuelCastReq) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok || p.duel == nil {
			return
		}
		d := p.duel
		me, them := d.sidesOf(p)
		a := classes.AbilityAt(classes.ID(p.ent.Class), req.Slot)
		if a == nil || me.energy < a.Cost || (a.Once && me.used[a.ID]) || !canCast(a.ID, me, them, req) {
			// The client thought it could, set it straight
			w.sendDuelState(d, me)
			return
		}
		if a.Target == protocol.TargetWord {
			// Reshape Reality: pick a word first, it's paid for then
			me.reshape = w.reshapeOptions(them)
			w.send(c, protocol.ServerDuelReshapeOptions, protocol.DuelReshapeOptions{Words: me.reshape})
			return
		}
		me.energy -= a.Cost
		if a.Once {
			me.used[a.ID] = true
		}
		w.cast(d, me, them, a.ID, req)
		w.syncDuel(d)
	})
}

// Rules for casting beyond having the energy
func canCast(id string, me, them *duelSide, req protocol.DuelCastReq) bool {
	switch id {
	case classes.ShieldsUp:
		return !me.shield
	case classes.Cripple:
		return them.unusedRows() >= 2
	case classes.Illusion:
		return them.illusion == nil
	case classes.Slash:
		t := tile{req.Row, req.Col}
		return t.row >= 0 && t.row < len(them.guesses) && t.col >= 0 && t.col < len(them.guesses[t.row].colors) && !them.destroyed[t]
	case classes.Scry:
		l := strings.ToUpper(req.Letter)
		return len(l) == 1 && l[0] >= 'A' && l[0] <= 'Z'
	}
	return true
}

// What each ability does, once it's paid for
func (w *World) cast(d *duel, me, them *duelSide, id string, req protocol.DuelCastReq) {
	now := w.now()
	switch id {
	case classes.Slash:
		t := tile{req.Row, req.Col}
		if w.hostile(d, me, them, id, protocol.CastUsed, t) {
			them.destroyed[t] = true
			w.reaimEyes(me, them)
		}
	case classes.ShieldsUp:
		me.shield = true
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
	case classes.Determination:
		me.rows++
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
	case classes.PommelStrike:
		if w.hostile(d, me, them, id, protocol.CastUsed, tile{}) {
			them.stun(now.Add(PommelStun))
		}
	case classes.Cripple:
		if w.hostile(d, me, them, id, protocol.CastUsed, tile{}) {
			them.rows--
		}
	case classes.Scry:
		letter := strings.ToUpper(req.Letter)
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
		w.send(me.p.client, protocol.ServerDuelScry, protocol.DuelScry{Letter: letter, InWord: strings.Contains(me.word, letter)})
	case classes.SeeingEye:
		if w.hostile(d, me, them, id, protocol.CastUsed, tile{}) {
			e := &eye{}
			w.aimEye(me, them, e)
			me.eyes = append(me.eyes, e)
			w.sendEyes(me, them)
		}
	case classes.MagicMissile:
		// The shield is checked when it lands
		w.castEvent(d, me, id, protocol.CastUsed, false, tile{})
		them.missiles = append(them.missiles, now.Add(MissileFlight))
	case classes.Illusion:
		if w.hostile(d, me, them, id, protocol.CastUsed, tile{}) {
			word := w.Duels.IllusionWord(w.rng)
			them.illusion = &illusionGame{word: word}
			w.send(them.p.client, protocol.ServerIllusionStart, protocol.IllusionStart{WordLength: len(word), MaxGuesses: IllusionGuesses})
		}
	}
}

// A Magic Missile from caster reaches target: they lose energy, and a row if
// it isn't their last
func (w *World) landMissile(d *duel, caster, target *duelSide) {
	if !w.hostile(d, caster, target, classes.MagicMissile, protocol.CastLanded, tile{}) {
		return
	}
	target.loseEnergy(MissileEnergy)
	if target.unusedRows() >= 2 {
		target.rows--
	}
}

// Points a Seeing Eye at a random letter in target's guesses it may see: not
// green, not destroyed, and not watched by another eye. Moves again in
// EyeInterval.
func (w *World) aimEye(watcher, target *duelSide, e *eye) {
	var options []tile
	for r, g := range target.guesses {
		for c, color := range g.colors {
			t := tile{r, c}
			if color == protocol.Green || target.destroyed[t] || watcher.watching(t, e) {
				continue
			}
			options = append(options, t)
		}
	}
	e.next = w.now().Add(EyeInterval)
	e.on = len(options) > 0
	if e.on {
		e.at = options[w.rng.Intn(len(options))]
	}
}

// Whether an eye other than except watches t
func (s *duelSide) watching(t tile, except *eye) bool {
	for _, e := range s.eyes {
		if e != except && e.on && e.at == t {
			return true
		}
	}
	return false
}

// Moves eyes whose letter turned green or was destroyed
func (w *World) reaimEyes(watcher, target *duelSide) {
	moved := false
	for _, e := range watcher.eyes {
		if e.on && (target.guesses[e.at.row].colors[e.at.col] == protocol.Green || target.destroyed[e.at]) {
			w.aimEye(watcher, target, e)
			moved = true
		}
	}
	if moved {
		w.sendEyes(watcher, target)
	}
}

// Tells watcher the letters their eyes see
func (w *World) sendEyes(watcher, target *duelSide) {
	msg := protocol.DuelEyes{Tiles: []protocol.EyeTile{}}
	for _, e := range watcher.eyes {
		if e.on {
			msg.Tiles = append(msg.Tiles, protocol.EyeTile{Row: e.at.row, Col: e.at.col, Letter: string(target.guesses[e.at.row].word[e.at.col])})
		}
	}
	w.send(watcher.p.client, protocol.ServerDuelEyes, msg)
}

// Words target's word could become: not their word, and nothing they've
// guessed, so a reshape never solves it for them
func (w *World) reshapeOptions(target *duelSide) []string {
	taken := map[string]bool{target.word: true}
	for _, g := range target.guesses {
		taken[g.word] = true
	}
	out := []string{}
	for tries := 0; len(out) < ReshapeOptions && tries < 100; tries++ {
		if word := w.Duels.NewWord(w.rng); !taken[word] {
			taken[word] = true
			out = append(out, word)
		}
	}
	return out
}

// The client picked a word for Reshape Reality: it becomes the opponent's word
// and their guesses are scored again
func (w *World) ReshapeReality(c Client, word string) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok || p.duel == nil {
			return
		}
		d := p.duel
		me, them := d.sidesOf(p)
		word = strings.ToUpper(word)
		cost := abilityCost(classes.ID(p.ent.Class), classes.ReshapeReality)
		if cost < 0 || !slices.Contains(me.reshape, word) || me.energy < cost {
			w.sendDuelState(d, me)
			return
		}
		me.reshape = nil
		me.energy -= cost
		if w.hostile(d, me, them, classes.ReshapeReality, protocol.CastUsed, tile{}) {
			them.word = word
			for i, g := range them.guesses {
				_, them.guesses[i].colors = w.Duels.Score(g.word, word)
			}
			them.relearn()
			colors := them.visibleColors()
			w.send(them.p.client, protocol.ServerDuelBoard, protocol.DuelBoard{Yours: true, Colors: colors})
			w.send(me.p.client, protocol.ServerDuelBoard, protocol.DuelBoard{Colors: colors})
			w.reaimEyes(me, them)
		}
		w.syncDuel(d)
	})
}

// What an ability of the class costs, -1 if the class doesn't have it
func abilityCost(class classes.ID, id string) int {
	for slot := range classes.AbilitySlots {
		if a := classes.AbilityAt(class, slot); a != nil && a.ID == id {
			return a.Cost
		}
	}
	return -1
}

// A guess in the Illusion the client is trapped in. Solving it frees them,
// failing it costs energy too.
func (w *World) IllusionGuess(c Client, guess string) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok || p.duel == nil {
			return
		}
		d := p.duel
		me, them := d.sidesOf(p)
		ill := me.illusion
		if ill == nil {
			return
		}
		res := protocol.WordleRes{Status: protocol.WordleInGame}
		if w.now().Before(me.stunnedUntil) {
			res.Blocked = true
			w.send(c, protocol.ServerIllusionGuess, res)
			return
		}
		valid, colors := w.Duels.Score(guess, ill.word)
		if !valid {
			w.send(c, protocol.ServerIllusionGuess, res)
			return
		}
		ill.guesses++
		res.Valid = true
		res.Colors = colors
		won := solves(colors)
		if won {
			res.Status = protocol.WordleWin
		} else if ill.guesses >= IllusionGuesses {
			res.Status = protocol.WordleLose
		}
		w.send(c, protocol.ServerIllusionGuess, res)
		if res.Status == protocol.WordleInGame {
			return
		}

		me.illusion = nil
		end := protocol.IllusionEnd{Won: won, Solution: ill.word}
		kind := protocol.CastFizzled
		if !won {
			end.EnergyLost = me.loseEnergy(IllusionEnergy)
			kind = protocol.CastLanded
		}
		w.send(c, protocol.ServerIllusionEnd, end)
		w.castEvent(d, them, classes.Illusion, kind, false, tile{})
		w.syncDuel(d)
	})
}

// A system: energy over time, stuns wearing off, missiles landing and Seeing
// Eyes moving
func (w *World) tickDuels() {
	now := w.now()
	for d := range w.duels {
		changed := false
		for !now.Before(d.nextEnergy) {
			for _, s := range d.sides {
				s.energy++
			}
			d.nextEnergy = d.nextEnergy.Add(EnergyInterval)
			changed = true
		}
		for _, s := range d.sides {
			_, other := d.sidesOf(s.p)
			if !s.stunnedUntil.IsZero() && !now.Before(s.stunnedUntil) {
				s.stunnedUntil = time.Time{}
				changed = true
			}
			if len(s.missiles) > 0 {
				flying := s.missiles[:0]
				for _, at := range s.missiles {
					if now.Before(at) {
						flying = append(flying, at)
						continue
					}
					w.landMissile(d, other, s)
					changed = true
				}
				s.missiles = flying
			}
			moved := false
			for _, e := range s.eyes {
				if !now.Before(e.next) {
					w.aimEye(s, other, e)
					moved = true
				}
			}
			if moved {
				w.sendEyes(s, other)
			}
		}
		if changed {
			w.syncDuel(d)
		}
	}
}
