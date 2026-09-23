package main

import (
	"nytrpg/resources"
	"reflect"
	"testing"
	"time"
)

func guessables(words ...string) *map[string]bool {
	m := make(map[string]bool)
	for _, w := range words {
		m[w] = true
	}
	return &m
}

func TestColorMyBoxes(t *testing.T) {
	g := guessables("crane", "speed", "abide", "eerie", "geese", "toolong")
	tests := []struct {
		guess, word string
		valid bool
		colors []WordleColor
	}{
		{"CRANE", "CRANE", true, []WordleColor{GREEN, GREEN, GREEN, GREEN, GREEN}},
		// lower case guesses work too
		{"crane", "CRANE", true, []WordleColor{GREEN, GREEN, GREEN, GREEN, GREEN}},
		// one E in the word: only the first E gets the yellow
		{"SPEED", "ABIDE", true, []WordleColor{GREY, GREY, YELLOW, GREY, YELLOW}},
		{"GEESE", "EERIE", true, []WordleColor{GREY, GREEN, YELLOW, GREY, GREEN}},
		// greens are counted before yellows
		{"EERIE", "GEESE", true, []WordleColor{YELLOW, GREEN, GREY, GREY, GREEN}},
		{"ZZZZZ", "CRANE", false, nil},
		{"TOOLONG", "CRANE", false, nil},
		{"", "CRANE", false, nil},
	}
	for _, tt := range tests {
		valid, colors := colorMyBoxes(tt.guess, tt.word, g)
		if valid != tt.valid || !reflect.DeepEqual(colors, tt.colors) {
			t.Errorf("colorMyBoxes(%q, %q) = %v, %v; want %v, %v", tt.guess, tt.word, valid, colors, tt.valid, tt.colors)
		}
	}
}

func loadedResources(t *testing.T) *resources.ResourceManager {
	rm := resources.NewResourceManager()
	rm.Load()
	return rm
}

func TestWordleForIsStable(t *testing.T) {
	a := loadedResources(t)
	b := loadedResources(t)
	for _, date := range []string{"2026-01-01", "2026-09-23", "2027-12-31"} {
		if a.WordleFor(date) != b.WordleFor(date) {
			t.Errorf("WordleFor(%s) differs between loads", date)
		}
	}
	if a.WordleFor("2026-09-23") == a.WordleFor("2026-09-24") && a.WordleFor("2026-09-24") == a.WordleFor("2026-09-25") {
		t.Errorf("WordleFor gives the same word three days running")
	}
}

func TestWordleSession(t *testing.T) {
	rm := loadedResources(t)
	notPlayed := func(string) bool { return false }
	start := time.Date(2026, 9, 23, 12, 0, 0, 0, resources.GameZone)
	word := rm.WordleFor(resources.DateOf(start))
	wrong := "CRANE"
	if word == wrong {
		wrong = "SLATE"
	}

	ws := newWordleSessions()

	// No session yet, guess is rejected
	if res, _ := ws.guess(1, word, start, rm, notPlayed); res.Valid {
		t.Fatal("guess without a session should be invalid")
	}

	ws.start(1, start)
	res, fin := ws.guess(1, wrong, start.Add(10*time.Second), rm, notPlayed)
	if !res.Valid || res.Status != INGAME || fin != nil || res.Solution != "" {
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

	for i := 2; i < GUESSES_ALLOWED; i++ {
		ws.guess(1, wrong, start, rm, notPlayed)
	}
	res, fin = ws.guess(1, wrong, start.Add(time.Minute), rm, notPlayed)
	if res.Status != LOSE || fin == nil || res.Solution != word || res.Seconds != 60 {
		t.Fatalf("last guess should lose: %+v %v", res, fin)
	}
	if len(fin.guesses) != GUESSES_ALLOWED {
		t.Fatalf("finished with %d guesses", len(fin.guesses))
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
	if res.Status != WIN || fin == nil {
		t.Fatalf("correct guess should win: %+v", res)
	}
}
