package wordle

import (
	"math/rand"
	"reflect"
	"testing"

	"nytrpg/internal/protocol"
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

func TestDuelPuzzleWithAFixedWord(t *testing.T) {
	p := NewDuelPuzzle(LoadWords()).WithWord("CRANE")
	rng := rand.New(rand.NewSource(1))
	for range 5 {
		if w := p.NewWord(rng); w != "CRANE" {
			t.Fatalf("got %s", w)
		}
	}
	if w := p.IllusionWord(rng); len(w) != 3 {
		t.Fatalf("illusions stay random 3-letter words, got %s", w)
	}
}

func TestIllusionWords(t *testing.T) {
	words := LoadWords()
	p := NewDuelPuzzle(words)
	rng := rand.New(rand.NewSource(1))
	seen := make(map[string]bool)
	for range 50 {
		w := p.IllusionWord(rng)
		if len(w) != 3 || !words.Guessable(w) {
			t.Fatalf("illusion word %q", w)
		}
		seen[w] = true
	}
	if len(seen) < 10 {
		t.Fatalf("only %d different illusion words", len(seen))
	}

	valid, colors := p.Score("act", "CAT")
	want := []protocol.WordleColor{protocol.Yellow, protocol.Yellow, protocol.Green}
	if !valid || !reflect.DeepEqual(colors, want) {
		t.Fatalf("Score = %v %v, want %v", valid, colors, want)
	}
	if valid, _ := p.Score("qzx", "CAT"); valid {
		t.Fatal("3-letter non-words must be invalid")
	}
	// The daily wordle's 5-letter words don't take 3-letter guesses
	if valid, _ := Score("cat", "CRANE", words); valid {
		t.Fatal("a 3-letter guess on a 5-letter word")
	}
}
