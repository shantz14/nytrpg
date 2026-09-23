package main

import (
	"encoding/json"
	"log"
	"net/http"
	"nytrpg/resources"
	"strconv"
	"strings"
	"sync"
	"time"
)

const GUESSES_ALLOWED = 5

// Scores a guess against the word. Returns false if the guess isn't a guessable word.
func colorMyBoxes(guess string, word string, guessables *map[string]bool) (bool, []WordleColor) {
	guess = strings.ToUpper(guess)
	if len(guess) != len(word) {
		return false, nil
	}
	if _, ok := (*guessables)[strings.ToLower(guess)]; !ok {
		return false, nil
	}

	letterCounts := countLetters(word)
	colors := make([]WordleColor, len(word))

	lettersCounted := make(map[rune]int)
	for _, letter := range guess {
		lettersCounted[letter] = 0
	}

	//greys
	for i := range guess {
		colors[i] = GREY
	}
	//greens
	for i, letter := range guess {
		if (guess[i] == word[i]) {
			colors[i] = GREEN
			lettersCounted[letter] += 1
		}
	}
	//yellows
	for i, letter := range guess {
		if (lettersCounted[letter] < letterCounts[letter] && colors[i] == GREY) {
			colors[i] = YELLOW
			lettersCounted[letter]++
		}
	}

	return true, colors
}

func allGreen(colors []WordleColor) bool {
	for _, c := range colors {
		if c != GREEN {
			return false
		}
	}
	return true
}

// Server side record of a player's wordle for one day. Lives across reconnects
// so the clock and guess count can't be reset by the client.
type WordleSession struct {
	date string
	start time.Time
	guesses []string
	colors [][]WordleColor
	done bool
}

type WordleSessions struct {
	mu sync.Mutex
	sessions map[int]*WordleSession
}

func newWordleSessions() *WordleSessions {
	return &WordleSessions{sessions: make(map[int]*WordleSession)}
}

// Starts the clock for today, unless it's already running. Returns the guesses so
// far so a reloaded client can pick up where it left off.
func (ws *WordleSessions) start(pid int, now time.Time) WordleResume {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	today := resources.DateOf(now)
	s, ok := ws.sessions[pid]
	if !ok || s.date != today {
		s = &WordleSession{date: today, start: now}
		ws.sessions[pid] = s
	}
	return WordleResume{
		Guesses: append([]string{}, s.guesses...),
		Colors: append([][]WordleColor{}, s.colors...),
		Seconds: now.Sub(s.start).Seconds(),
	}
}

// Scores a guess for the player's current session. finished is set once the game
// is won or lost, the caller should then record the result.
func (ws *WordleSessions) guess(pid int, guess string, now time.Time, rm *resources.ResourceManager, played func(date string) bool) (res WordleRes, finished *WordleSession) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	res.Status = INGAME
	s, ok := ws.sessions[pid]
	if !ok || s.done {
		return res, nil
	}
	if played(s.date) {
		return res, nil
	}

	word := rm.WordleFor(s.date)
	valid, colors := colorMyBoxes(guess, word, &rm.GuessableWords)
	if !valid {
		return res, nil
	}
	s.guesses = append(s.guesses, strings.ToUpper(guess))
	s.colors = append(s.colors, colors)

	res.Valid = true
	res.Colors = colors
	res.Seconds = now.Sub(s.start).Seconds()

	if allGreen(colors) {
		res.Status = WIN
	} else if len(s.guesses) >= GUESSES_ALLOWED {
		res.Status = LOSE
	}
	if res.Status != INGAME {
		s.done = true
		res.Solution = word
		copied := *s
		return res, &copied
	}
	return res, nil
}

func countLetters(word string) map[rune]int {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterCounts := make(map[rune]int)
	for _, letter := range letters {
		letterCounts[letter] = 0
	}
	for _, letter := range word {
		letterCounts[letter] += 1
	}
	return letterCounts
}

func handleHaveIPlayed(h *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed, only GET allowed.", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr); if err != nil {
		http.Error(w, "Bad id.", http.StatusBadRequest)
		return
	}
	res, err := h.db.playedOn(id, resources.Today())
	if err != nil {
		http.Error(w, "Database error.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error encoding haveiplayed response:", err)
		return
	}
}

type WordleStatus int

const (
	INGAME WordleStatus = 0
	WIN WordleStatus = 1
	LOSE WordleStatus = 2
)

type WordleColor int

const (
	GREY WordleColor = 0
	YELLOW WordleColor = 1
	GREEN WordleColor = 2
)

