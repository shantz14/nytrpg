package wordle

import (
	"math/rand"

	"nytrpg/internal/protocol"
)

// Wordle as the puzzle for duels (game.DuelPuzzle): a random word each duel,
// nothing to do with the daily one or the leaderboard
type DuelPuzzle struct {
	words *Words
	// Every word is this one when set, for end-to-end tests
	fixed string
}

func NewDuelPuzzle(words *Words) DuelPuzzle {
	return DuelPuzzle{words: words}
}

// Always word, so tests can plan their guesses. Reshape Reality has nothing
// to offer then: every other word it could pick is the same one.
func (p DuelPuzzle) WithWord(word string) DuelPuzzle {
	p.fixed = word
	return p
}

func (p DuelPuzzle) NewWord(rng *rand.Rand) string {
	if p.fixed != "" {
		return p.fixed
	}
	return p.words.Random(rng)
}

// A 3-letter word for an Illusion. Score works on it like any word.
func (p DuelPuzzle) IllusionWord(rng *rand.Rand) string {
	return p.words.Three(rng)
}

func (p DuelPuzzle) Score(guess, word string) (bool, []protocol.WordleColor) {
	return Score(guess, word, p.words)
}

func (p DuelPuzzle) ScoreAny(guess, word string) (bool, []protocol.WordleColor) {
	return ScoreAny(guess, word)
}

func (p DuelPuzzle) MaxGuesses() int {
	return GuessesAllowed
}
