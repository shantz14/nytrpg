package wordle

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"nytrpg/internal/gameday"
	. "nytrpg/internal/protocol"
)

func testWords(words ...string) *Words {
	w := &Words{guessables: make(map[string]bool)}
	for _, word := range words {
		w.guessables[strings.ToUpper(word)] = true
	}
	return w
}

func TestColorMyBoxes(t *testing.T) {
	g := testWords("crane", "speed", "abide", "eerie", "geese", "toolong")
	tests := []struct {
		guess, word string
		valid       bool
		colors      []WordleColor
	}{
		{"CRANE", "CRANE", true, []WordleColor{Green, Green, Green, Green, Green}},
		// lower case guesses work too
		{"crane", "CRANE", true, []WordleColor{Green, Green, Green, Green, Green}},
		// one E in the word: only the first E gets the yellow
		{"SPEED", "ABIDE", true, []WordleColor{Grey, Grey, Yellow, Grey, Yellow}},
		{"GEESE", "EERIE", true, []WordleColor{Grey, Green, Yellow, Grey, Green}},
		// greens are counted before yellows
		{"EERIE", "GEESE", true, []WordleColor{Yellow, Green, Grey, Grey, Green}},
		{"ZZZZZ", "CRANE", false, nil},
		{"TOOLONG", "CRANE", false, nil},
		{"", "CRANE", false, nil},
	}
	for _, tt := range tests {
		valid, colors := Score(tt.guess, tt.word, g)
		if valid != tt.valid || !reflect.DeepEqual(colors, tt.colors) {
			t.Errorf("Score(%q, %q) = %v, %v; want %v, %v", tt.guess, tt.word, valid, colors, tt.valid, tt.colors)
		}
	}
}

func loadedResources(t *testing.T) *Words {
	return LoadWords()
}

func TestWordleForIsStable(t *testing.T) {
	a := loadedResources(t)
	b := loadedResources(t)
	for _, date := range []string{"2026-01-01", "2026-09-23", "2027-12-31"} {
		if a.For(date) != b.For(date) {
			t.Errorf("WordleFor(%s) differs between loads", date)
		}
	}
	if a.For("2026-09-23") == a.For("2026-09-24") && a.For("2026-09-24") == a.For("2026-09-25") {
		t.Errorf("WordleFor gives the same word three days running")
	}
}

func TestWordleSession(t *testing.T) {
	rm := loadedResources(t)
	notPlayed := func(string) bool { return false }
	start := time.Date(2026, 9, 23, 12, 0, 0, 0, gameday.Zone)
	word := rm.For(gameday.Of(start))
	wrong := "CRANE"
	if word == wrong {
		wrong = "SLATE"
	}

	ws := newSessions()

	// No session yet, guess is rejected
	if res, _ := ws.guess(1, word, start, rm, notPlayed); res.Valid {
		t.Fatal("guess without a session should be invalid")
	}

	ws.start(1, start)
	res, fin := ws.guess(1, wrong, start.Add(10*time.Second), rm, notPlayed)
	if !res.Valid || res.Status != WordleInGame || fin != nil || res.Solution != "" {
		t.Fatalf("first wrong guess: %+v %v", res, fin)
	}

	// Starting again (reload) keeps the clock and guesses
	resume := ws.start(1, start.Add(20*time.Second))
	if len(resume.Guesses) != 1 || resume.Seconds != 20 {
		t.Fatalf("resume = %+v", resume)
	}

	// Invalid words don't use up a guess
	if res, _ := ws.guess(1, "ZZZZZ", start, rm, notPlayed); res.Valid {
		t.Fatal("ZZZZZ should be invalid")
	}

	for i := 2; i < GuessesAllowed; i++ {
		ws.guess(1, wrong, start, rm, notPlayed)
	}
	res, fin = ws.guess(1, wrong, start.Add(time.Minute), rm, notPlayed)
	if res.Status != WordleLose || fin == nil || res.Solution != word || res.Seconds != 60 {
		t.Fatalf("last guess should lose: %+v %v", res, fin)
	}
	if fin.Guesses != GuessesAllowed {
		t.Fatalf("finished with %d guesses", fin.Guesses)
	}

	// Done, nothing else is accepted
	if res, _ := ws.guess(1, word, start, rm, notPlayed); res.Valid {
		t.Fatal("guess after game over should be invalid")
	}

	// A different player can win, and already having played blocks guesses
	ws.start(2, start)
	if res, _ := ws.guess(2, word, start, rm, func(string) bool { return true }); res.Valid {
		t.Fatal("guess when already played should be invalid")
	}
	res, fin = ws.guess(2, word, start, rm, notPlayed)
	if res.Status != WordleWin || fin == nil {
		t.Fatalf("correct guess should win: %+v", res)
	}
}
