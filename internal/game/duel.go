package game

import (
	"math/rand"
	"time"

	"nytrpg/internal/protocol"
)

// How long a challenge waits for an answer
const ChallengeTimeout = 30 * time.Second

// The puzzle both duelists race to solve. The world doesn't know about Wordle,
// the server plugs it in.
type DuelPuzzle interface {
	// A fresh word for one duel
	NewWord(rng *rand.Rand) string
	// Scores a guess against the word, valid is false if it isn't a guessable word
	Score(guess, word string) (valid bool, colors []protocol.WordleColor)
	MaxGuesses() int
}

// A challenge waiting for the target to accept or deny
type challenge struct {
	id       uint32
	from, to *player
	expires  time.Time
}

// One player's half of a duel
type duelSide struct {
	p       *player
	guesses int
	solved  bool
	// Used every guess without solving. They wait for the other side to finish.
	out bool
}

func (s *duelSide) done() bool { return s.solved || s.out }

// Two players racing on the same word. Both players point at it.
type duel struct {
	word  string
	start time.Time
	sides [2]*duelSide
}

// The player's side and their opponent's
func (d *duel) sidesOf(p *player) (me, them *duelSide) {
	if d.sides[0].p == p {
		return d.sides[0], d.sides[1]
	}
	return d.sides[1], d.sides[0]
}

// What other players call them: the character name, or the username
func (p *player) displayName() string {
	if p.ent.Char != "" {
		return p.ent.Char
	}
	return p.ent.Name
}

func (w *World) playerByEntity(id protocol.EntityID) *player {
	for _, p := range w.players {
		if p.ent.ID == id {
			return p
		}
	}
	return nil
}

func (w *World) challengeUpdate(p *player, id uint32, other *player, status protocol.DuelChallengeStatus) {
	w.send(p.client, protocol.ServerDuelChallengeUpdate, protocol.DuelChallengeUpdate{ID: id, Name: other.displayName(), Status: status})
}

// Challenges the player with entity id target to a duel. They must be in view
// and neither of you can already be dueling. A new challenge replaces your
// pending one.
func (w *World) Challenge(c Client, target protocol.EntityID) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok {
			return
		}
		t := w.playerByEntity(target)
		_, inView := p.known[target]
		if t == nil || t == p || !inView || w.Duels == nil {
			name := ""
			if t != nil {
				name = t.displayName()
			}
			w.send(c, protocol.ServerDuelChallengeUpdate, protocol.DuelChallengeUpdate{Name: name, Status: protocol.DuelUnavailable})
			return
		}
		if p.duel != nil || t.duel != nil {
			w.challengeUpdate(p, 0, t, protocol.DuelBusy)
			return
		}
		for id, ch := range w.challenges {
			if ch.from != p {
				continue
			}
			if ch.to == t {
				// Already waiting on them
				return
			}
			delete(w.challenges, id)
			w.challengeUpdate(ch.to, id, p, protocol.DuelCancelled)
		}

		w.nextChallenge++
		ch := &challenge{id: w.nextChallenge, from: p, to: t, expires: w.now().Add(ChallengeTimeout)}
		w.challenges[ch.id] = ch
		w.send(t.client, protocol.ServerDuelChallenge, protocol.DuelChallenge{
			ID:        ch.id,
			From:      p.ent.ID,
			Name:      p.ent.Name,
			Char:      p.ent.Char,
			Class:     p.ent.Class,
			ExpiresMs: int(ChallengeTimeout / time.Millisecond),
		})
		w.challengeUpdate(p, ch.id, t, protocol.DuelSent)
	})
}

// Accepts or denies a challenge. Only its target can answer it.
func (w *World) RespondDuel(c Client, id uint32, accept bool) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		ch := w.challenges[id]
		if !ok || ch == nil || ch.to != p {
			return
		}
		delete(w.challenges, id)
		if !accept {
			w.challengeUpdate(ch.from, id, p, protocol.DuelDeclined)
			return
		}
		if p.duel != nil || ch.from.duel != nil {
			w.challengeUpdate(ch.from, id, p, protocol.DuelBusy)
			return
		}
		w.startDuel(ch.from, p)
	})
}

func (w *World) startDuel(a, b *player) {
	// Nobody can accept a challenge from someone who's now busy
	w.cancelChallenges(a)
	w.cancelChallenges(b)

	d := &duel{
		word:  w.Duels.NewWord(w.rng),
		start: w.now(),
		sides: [2]*duelSide{{p: a}, {p: b}},
	}
	a.duel = d
	b.duel = d
	for _, s := range d.sides {
		_, them := d.sidesOf(s.p)
		w.send(s.p.client, protocol.ServerDuelStart, protocol.DuelStart{
			Name:       them.p.ent.Name,
			Char:       them.p.ent.Char,
			Class:      them.p.ent.Class,
			WordLength: len(d.word),
			MaxGuesses: w.Duels.MaxGuesses(),
		})
	}
}

// Drops every challenge to or from p, telling the other player
func (w *World) cancelChallenges(p *player) {
	for id, ch := range w.challenges {
		switch p {
		case ch.from:
			w.challengeUpdate(ch.to, id, p, protocol.DuelCancelled)
		case ch.to:
			w.challengeUpdate(ch.from, id, p, protocol.DuelCancelled)
		default:
			continue
		}
		delete(w.challenges, id)
	}
}

// A system: challenges nobody answered in time
func (w *World) expireChallenges() {
	now := w.now()
	for id, ch := range w.challenges {
		if now.Before(ch.expires) {
			continue
		}
		delete(w.challenges, id)
		w.challengeUpdate(ch.from, id, ch.to, protocol.DuelExpired)
		w.challengeUpdate(ch.to, id, ch.from, protocol.DuelExpired)
	}
}

// Scores a guess in the client's duel. The opponent sees its colors right away,
// never its letters.
func (w *World) DuelGuess(c Client, guess string) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok || p.duel == nil {
			return
		}
		d := p.duel
		me, them := d.sidesOf(p)
		if me.done() {
			return
		}
		res := protocol.WordleRes{Status: protocol.WordleInGame}
		valid, colors := w.Duels.Score(guess, d.word)
		if !valid {
			w.send(c, protocol.ServerDuelGuess, res)
			return
		}
		me.guesses++
		res.Valid = true
		res.Colors = colors
		res.Seconds = w.now().Sub(d.start).Seconds()
		if solves(colors) {
			me.solved = true
			res.Status = protocol.WordleWin
		} else if me.guesses >= w.Duels.MaxGuesses() {
			me.out = true
			res.Status = protocol.WordleLose
		}
		w.send(c, protocol.ServerDuelGuess, res)
		w.send(them.p.client, protocol.ServerDuelOpponentGuess, protocol.DuelOpponentGuess{Colors: colors})

		switch {
		case me.solved:
			w.endDuel(d, me, protocol.DuelSolved)
		case me.out && them.out:
			w.endDuel(d, nil, protocol.DuelOutOfGuesses)
		}
	})
}

// A guess that's all green is the word
func solves(colors []protocol.WordleColor) bool {
	for _, c := range colors {
		if c != protocol.Green {
			return false
		}
	}
	return len(colors) > 0
}

// Tells the opponent how many letters are in the client's current row
func (w *World) DuelTyping(c Client, count int) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok || p.duel == nil {
			return
		}
		me, them := p.duel.sidesOf(p)
		if me.done() || count < 0 || count > len(p.duel.word) {
			return
		}
		w.send(them.p.client, protocol.ServerDuelTyping, protocol.DuelTyping{Count: count})
	})
}

// Gives up the client's duel, the opponent wins
func (w *World) ForfeitDuel(c Client) {
	w.Do(func(w *World) {
		if p, ok := w.players[c]; ok && p.duel != nil {
			_, them := p.duel.sidesOf(p)
			w.endDuel(p.duel, them, protocol.DuelForfeit)
		}
	})
}

// Ends the duel for both players. winner nil is a draw.
func (w *World) endDuel(d *duel, winner *duelSide, reason protocol.DuelEndReason) {
	secs := w.now().Sub(d.start).Seconds()
	for _, s := range d.sides {
		outcome := protocol.DuelLose
		if winner == nil {
			outcome = protocol.DuelDraw
		} else if winner == s {
			outcome = protocol.DuelWin
		}
		w.send(s.p.client, protocol.ServerDuelEnd, protocol.DuelEnd{Outcome: outcome, Reason: reason, Solution: d.word, Seconds: secs})
		s.p.duel = nil
	}
}

// A player is leaving: they lose their duel and their challenges are off
func (w *World) leaveDuels(p *player) {
	if p.duel != nil {
		_, them := p.duel.sidesOf(p)
		w.endDuel(p.duel, them, protocol.DuelDisconnect)
	}
	w.cancelChallenges(p)
}
