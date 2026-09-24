package wordle

import (
	"strings"
	"sync"
	"time"

	"nytrpg/internal/gameday"
	"nytrpg/internal/protocol"
)

const GuessesAllowed = 5

// Scores a guess against the word. Returns false if the guess isn't a guessable word.
func Score(guess string, word string, words *Words) (bool, []protocol.WordleColor) {
	guess = strings.ToUpper(guess)
	if len(guess) != len(word) || !words.Guessable(guess) {
		return false, nil
	}

	letterCounts := make(map[rune]int)
	for _, letter := range word {
		letterCounts[letter]++
	}
	colors := make([]protocol.WordleColor, len(word))
	lettersCounted := make(map[rune]int)

	//greens
	for i, letter := range guess {
		if guess[i] == word[i] {
			colors[i] = protocol.Green
			lettersCounted[letter]++
		}
	}
	//yellows
	for i, letter := range guess {
		if colors[i] == protocol.Grey && lettersCounted[letter] < letterCounts[letter] {
			colors[i] = protocol.Yellow
			lettersCounted[letter]++
		}
	}

	return true, colors
}

func allGreen(colors []protocol.WordleColor) bool {
	for _, c := range colors {
		if c != protocol.Green {
			return false
		}
	}
	return true
}

// Server side record of a player's wordle for one day. Lives across reconnects
// so the clock and guess count can't be reset by the client.
type session struct {
	date    string
	start   time.Time
	guesses []string
	colors  [][]protocol.WordleColor
	done    bool
}

// A finished game, ready to save
type Finished struct {
	Date    string
	Win     bool
	Guesses int
	Seconds float64
}

type sessions struct {
	mu sync.Mutex
	// By player id: one game a day per account, whichever character plays
	sessions map[int]*session
}

func newSessions() *sessions {
	return &sessions{sessions: make(map[int]*session)}
}

// Starts the clock for today, unless it's already running. Returns the guesses so
// far so a reloaded client can pick up where it left off.
func (ss *sessions) start(pid int, now time.Time) protocol.WordleResume {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	today := gameday.Of(now)
	s, ok := ss.sessions[pid]
	if !ok || s.date != today {
		s = &session{date: today, start: now}
		ss.sessions[pid] = s
	}
	return protocol.WordleResume{
		Guesses: append([]string{}, s.guesses...),
		Colors:  append([][]protocol.WordleColor{}, s.colors...),
		Seconds: now.Sub(s.start).Seconds(),
	}
}

// Scores a guess for the player's current session. finished is set once the game
// is won or lost, the caller should then record the result.
func (ss *sessions) guess(pid int, guess string, now time.Time, words *Words, played func(date string) bool) (res protocol.WordleRes, finished *Finished) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	res.Status = protocol.WordleInGame
	s, ok := ss.sessions[pid]
	if !ok || s.done || played(s.date) {
		return res, nil
	}

	word := words.For(s.date)
	valid, colors := Score(guess, word, words)
	if !valid {
		return res, nil
	}
	s.guesses = append(s.guesses, strings.ToUpper(guess))
	s.colors = append(s.colors, colors)

	res.Valid = true
	res.Colors = colors
	res.Seconds = now.Sub(s.start).Seconds()

	if allGreen(colors) {
		res.Status = protocol.WordleWin
	} else if len(s.guesses) >= GuessesAllowed {
		res.Status = protocol.WordleLose
	}
	if res.Status == protocol.WordleInGame {
		return res, nil
	}
	s.done = true
	res.Solution = word
	return res, &Finished{Date: s.date, Win: res.Status == protocol.WordleWin, Guesses: len(s.guesses), Seconds: res.Seconds}
}
