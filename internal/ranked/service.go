package ranked

import (
	"log/slog"
	"sync"
	"time"

	"nytrpg/internal/protocol"
	"nytrpg/internal/store"
)

// A settled ranked duel, ready to save
type Match struct {
	PlayedAt time.Time
	// Character ids
	A, B    int
	Outcome Outcome
	Reason  int
	Result  Result
	// Guesses each had made when it ended
	AGuesses int
	BGuesses int
	Seconds  float64
}

// Loads and saves ratings. Every read and write runs in order on one
// goroutine, so a character's rating loaded after a duel was recorded always
// includes it. Record never blocks, so the world can call it.
type Service struct {
	store *store.Store

	mu     sync.Mutex
	queue  []func()
	wake   chan struct{}
	closed bool
	done   chan struct{}
}

func NewService(st *store.Store) *Service {
	s := &Service{store: st, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go s.run()
	return s
}

func (s *Service) run() {
	defer close(s.done)
	for {
		s.mu.Lock()
		if len(s.queue) == 0 {
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			<-s.wake
			continue
		}
		job := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()
		job()
	}
}

func (s *Service) enqueue(job func()) {
	s.mu.Lock()
	s.queue = append(s.queue, job)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Runs job after everything queued before it, and waits for it
func (s *Service) wait(job func()) {
	done := make(chan struct{})
	s.enqueue(func() {
		defer close(done)
		job()
	})
	<-done
}

// Saves everything queued, then stops. Call after the world has stopped.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	<-s.done
}

func fromStore(r store.Rating) Rating {
	return Rating{Elo: r.Elo, Peak: r.Peak, Games: r.Games, Wins: r.Wins, Losses: r.Losses, Draws: r.Draws}
}

func toStore(charID int, r Rating) store.Rating {
	return store.Rating{CharacterID: charID, Elo: r.Elo, Peak: r.Peak, Games: r.Games, Wins: r.Wins, Losses: r.Losses, Draws: r.Draws}
}

// The character's rating, including every duel recorded before this call
func (s *Service) Load(charID int) Rating {
	r := NewRating()
	s.wait(func() {
		stored, found, err := s.store.Rating(charID)
		if err != nil {
			slog.Error("loading rating", "character", charID, "err", err)
		}
		if found {
			r = fromStore(stored)
		}
	})
	return r
}

// Saves a duel and both new ratings, in the background
func (s *Service) Record(m Match) {
	s.enqueue(func() {
		d := store.RankedDuel{
			PlayedAt: m.PlayedAt.Unix(),
			A:        m.A,
			B:        m.B,
			Reason:   m.Reason,
			ABefore:  m.Result.A.Elo - m.Result.DeltaA,
			AAfter:   m.Result.A.Elo,
			BBefore:  m.Result.B.Elo - m.Result.DeltaB,
			BAfter:   m.Result.B.Elo,
			AGuesses: m.AGuesses,
			BGuesses: m.BGuesses,
			Seconds:  m.Seconds,
		}
		switch m.Outcome {
		case AWins:
			d.Winner = m.A
		case BWins:
			d.Winner = m.B
		}
		if err := s.store.ApplyRankedDuel(d, toStore(m.A, m.Result.A), toStore(m.B, m.Result.B)); err != nil {
			slog.Error("saving ranked duel", "a", m.A, "b", m.B, "err", err)
		}
	})
}

// How many recent duels a profile shows
const RecentDuels = 5

// The character's latest ranked duels, newest first, including every duel
// recorded before this call
func (s *Service) Recent(charID int) []protocol.RankedMatchInfo {
	out := []protocol.RankedMatchInfo{}
	s.wait(func() {
		rows, err := s.store.RecentRankedDuels(charID, RecentDuels)
		if err != nil {
			slog.Error("loading ranked duels", "character", charID, "err", err)
		}
		for _, r := range rows {
			m := protocol.RankedMatchInfo{Opponent: r.Opponent, OpponentClass: r.OpponentClass, Outcome: protocol.DuelLose, Change: r.Change, PlayedAt: r.PlayedAt}
			if r.Won {
				m.Outcome = protocol.DuelWin
			} else if r.Draw {
				m.Outcome = protocol.DuelDraw
			}
			out = append(out, m)
		}
	})
	return out
}
