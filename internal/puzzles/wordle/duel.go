package wordle

import (
	"math/rand"

	"nytrpg/internal/protocol"
)

// Wordle as the puzzle for duels (game.DuelPuzzle): a random word each duel,
// nothing to do with the daily one or the leaderboard
type DuelPuzzle struct {
	words *Words
}

func NewDuelPuzzle(words *Words) DuelPuzzle {
	return DuelPuzzle{words: words}
}

func (p DuelPuzzle) NewWord(rng *rand.Rand) string {
	return p.words.Random(rng)
}

func (p DuelPuzzle) Score(guess, word string) (bool, []protocol.WordleColor) {
	return Score(guess, word, p.words)
}

func (p DuelPuzzle) MaxGuesses() int {
	return GuessesAllowed
}
