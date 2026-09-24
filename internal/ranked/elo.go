// Package ranked is the ranked duel ladder: elo ratings, how a ranked duel
// changes them, and the ranks elo maps to. This file is pure math; service.go
// saves ratings.
package ranked

import (
	"math"

	"nytrpg/internal/protocol"
)

const (
	// Where every character starts: Silver 3
	StartElo = 1000
	// How far one duel can move a rating, before the margin
	K = 32
	// New characters move faster so they find their rank quickly
	ProvisionalK     = 48
	ProvisionalGames = 10
	// The margin multiplier's range. A forfeit is always the most.
	MinMargin = 1.0
	MaxMargin = 1.75
	// Solving this fast or faster counts fully toward the margin, slower counts less
	fastSolveSeconds = 180
)

// A character's ranked record
type Rating struct {
	Elo    int
	Peak   int
	Games  int
	Wins   int
	Losses int
	Draws  int
}

func NewRating() Rating {
	return Rating{Elo: StartElo, Peak: StartElo}
}

func (r Rating) k() float64 {
	if r.Games < ProvisionalGames {
		return ProvisionalK
	}
	return K
}

// How likely a rated player is to beat b rated player, 0 to 1
func Expected(a, b int) float64 {
	return 1 / (1 + math.Pow(10, float64(b-a)/400))
}

// How decisive a win was, from the moment the winner solved it
type Margin struct {
	WinnerGuesses int
	WinnerSeconds float64
	// Guesses the loser had made, and whether that was all of them
	LoserGuesses int
	LoserOut     bool
	// How close the loser's best guess was, 0 to 1 (see Closeness)
	LoserBest float64
	// The loser forfeited or disconnected
	Forfeit bool
}

// The multiplier on the elo change, MinMargin to MaxMargin: more for winning in
// fewer guesses, against someone who wasn't close, and quickly
func (m Margin) Multiplier() float64 {
	if m.Forfeit {
		return MaxMargin
	}
	guessEdge := clamp(float64(m.LoserGuesses-m.WinnerGuesses+1)/3, 0, 1)
	if m.LoserOut {
		guessEdge = 1
	}
	speed := clamp(1-m.WinnerSeconds/fastSolveSeconds, 0, 1)
	mult := MinMargin + 0.3*guessEdge + 0.3*(1-clamp(m.LoserBest, 0, 1)) + 0.15*speed
	return clamp(mult, MinMargin, MaxMargin)
}

// How close a guess was: greens count fully, yellows half, over the word length
func Closeness(colors []protocol.WordleColor) float64 {
	if len(colors) == 0 {
		return 0
	}
	score := 0.0
	for _, c := range colors {
		switch c {
		case protocol.Green:
			score += 1
		case protocol.Yellow:
			score += 0.5
		}
	}
	return score / float64(len(colors))
}

type Outcome int

const (
	AWins Outcome = iota
	BWins
	Draw
)

// What a ranked duel did to both ratings
type Result struct {
	A, B Rating
	// Elo change for each, after the floor at 0
	DeltaA, DeltaB int
	// A's chance of winning going in
	ExpectedA float64
	// The margin multiplier used, 1 for a draw
	Multiplier float64
}

// Applies a ranked duel to both ratings. Between established players the
// change is zero-sum; each side uses its own K, so a new character moves more.
func Settle(a, b Rating, outcome Outcome, m Margin) Result {
	res := Result{A: a, B: b, ExpectedA: Expected(a.Elo, b.Elo), Multiplier: 1}
	scoreA := 0.5
	switch outcome {
	case AWins:
		scoreA = 1
		res.Multiplier = m.Multiplier()
	case BWins:
		scoreA = 0
		res.Multiplier = m.Multiplier()
	}
	decisive := outcome != Draw
	res.DeltaA = change(a, scoreA-res.ExpectedA, res.Multiplier, decisive)
	res.DeltaB = change(b, (1-scoreA)-(1-res.ExpectedA), res.Multiplier, decisive)

	res.A, res.DeltaA = apply(a, res.DeltaA, scoreA)
	res.B, res.DeltaB = apply(b, res.DeltaB, 1-scoreA)
	return res
}

// The elo change for one side. A decisive result always moves at least 1.
func change(r Rating, surprise, mult float64, decisive bool) int {
	d := int(math.Round(r.k() * mult * surprise))
	if decisive && d == 0 {
		if surprise > 0 {
			d = 1
		} else {
			d = -1
		}
	}
	return d
}

// Adds delta (floored so elo stays >= 0) and counts the game
func apply(r Rating, delta int, score float64) (Rating, int) {
	if r.Elo+delta < 0 {
		delta = -r.Elo
	}
	r.Elo += delta
	r.Peak = max(r.Peak, r.Elo)
	r.Games++
	switch score {
	case 1:
		r.Wins++
	case 0:
		r.Losses++
	default:
		r.Draws++
	}
	return r, delta
}

// What me could win and lose against them: the smallest and largest margins
func Stakes(me, them Rating) protocol.Stakes {
	e := Expected(me.Elo, them.Elo)
	return protocol.Stakes{
		WinMin:  change(me, 1-e, MinMargin, true),
		WinMax:  change(me, 1-e, MaxMargin, true),
		LoseMin: max(-me.Elo, change(me, -e, MinMargin, true)),
		LoseMax: max(-me.Elo, change(me, -e, MaxMargin, true)),
	}
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
