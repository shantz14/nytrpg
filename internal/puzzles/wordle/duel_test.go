package wordle

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestDuelPuzzle(t *testing.T) {
	words := LoadWords()
	p := NewDuelPuzzle(words)
	rng := rand.New(rand.NewSource(1))

	isSolution := make(map[string]bool)
	for _, s := range words.solutions {
		isSolution[s] = true
	}
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		w := p.NewWord(rng)
		if !isSolution[w] {
			t.Fatalf("%q isn't a solution", w)
		}
		seen[w] = true
	}
	if len(seen) < 10 {
		t.Fatalf("only %d different words in 50 duels", len(seen))
	}

	valid, colors := p.Score("slate", "CRANE")
	_, want := Score("slate", "CRANE", words)
	if !valid || !reflect.DeepEqual(colors, want) {
		t.Fatalf("Score = %v %v, want the daily scoring %v", valid, colors, want)
	}
	if valid, _ := p.Score("zzzzz", "CRANE"); valid {
		t.Fatal("non-words must be invalid")
	}
	if p.MaxGuesses() != GuessesAllowed {
		t.Fatal("duels get the usual number of guesses")
	}
}
