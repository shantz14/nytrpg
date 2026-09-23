// Package wordle is the daily Wordle puzzle: word lists, scoring, the
// server side game sessions, and its HTTP and websocket handlers.
package wordle

import (
	"bufio"
	_ "embed"
	"hash/fnv"
	"math/rand"
	"strings"
)

//go:embed words/all.txt
var allWords string

//go:embed words/solutions.txt
var solutionWords string

type Words struct {
	solutions  []string
	guessables map[string]bool
}

func LoadWords() *Words {
	w := &Words{guessables: make(map[string]bool)}
	for _, word := range lines(allWords) {
		w.guessables[strings.ToUpper(word)] = true
	}
	w.solutions = lines(strings.ToUpper(solutionWords))
	if len(w.solutions) == 0 {
		panic("wordle: no solutions")
	}
	return w
}

func lines(s string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		if word := strings.TrimSpace(scanner.Text()); word != "" {
			out = append(out, word)
		}
	}
	return out
}

// The word for a given date. Same date always gives the same word, even across restarts.
func (w *Words) For(date string) string {
	h := fnv.New64a()
	h.Write([]byte(date))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	return w.solutions[r.Intn(len(w.solutions))]
}

// guess must be upper case
func (w *Words) Guessable(guess string) bool {
	return w.guessables[guess]
}
