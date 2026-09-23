package wordle

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"nytrpg/internal/gameday"
	"nytrpg/internal/netconn"
	"nytrpg/internal/protocol"
	"nytrpg/internal/store"
)

const leaderboardPageSize = 10

type Service struct {
	words    *Words
	sessions *sessions
	store    *store.Store
}

func NewService(s *store.Store) *Service {
	w := LoadWords()
	slog.Info("wordle loaded", "today", w.For(gameday.Today()))
	return &Service{words: w, sessions: newSessions(), store: s}
}

func (svc *Service) RegisterHandlers(r *netconn.Router) {
	r.Handle(protocol.ClientWordleStart, func(s *netconn.Session, _ msgpack.RawMessage) {
		s.SendMsg(protocol.ServerWordleResume, svc.sessions.start(s.PlayerID, time.Now()))
	})
	r.Handle(protocol.ClientWordleGuess, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.WordleReq
		if err := msgpack.Unmarshal(data, &req); err != nil {
			return
		}
		s.SendMsg(protocol.ServerWordleResult, svc.guess(s.PlayerID, req.Guess))
	})
}

func (svc *Service) guess(pid int, guess string) protocol.WordleRes {
	played := func(date string) bool {
		played, err := svc.store.PlayedWordleOn(pid, date)
		if err != nil {
			slog.Error("checking if player played", "err", err)
		}
		// If the db is broken don't let them play
		return played || err != nil
	}
	res, finished := svc.sessions.guess(pid, guess, time.Now(), svc.words, played)
	if finished != nil {
		err := svc.store.InsertWordle(store.WordleResult{
			Date:       finished.Date,
			Win:        finished.Win,
			Seconds:    finished.Seconds,
			GuessCount: finished.Guesses,
			PlayerID:   pid,
		})
		if err != nil {
			slog.Error("insert wordle record", "err", err)
		}
	}
	return res
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "err", err)
	}
}

func (svc *Service) HandleHaveIPlayed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed, only GET allowed.", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "Bad id.", http.StatusBadRequest)
		return
	}
	played, err := svc.store.PlayedWordleOn(id, gameday.Today())
	if err != nil {
		slog.Error("checking if player played", "err", err)
		http.Error(w, "Database error.", http.StatusInternalServerError)
		return
	}
	writeJSON(w, played)
}

type leaderboardRes struct {
	Date     string                 `json:"date"`
	Today    string                 `json:"today"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int                    `json:"total"`
	Rows     []store.LeaderboardRow `json:"rows"`
}

func (svc *Service) HandleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed, only GET allowed.", http.StatusMethodNotAllowed)
		return
	}

	res := leaderboardRes{Today: gameday.Today(), PageSize: leaderboardPageSize}

	res.Date = r.URL.Query().Get("date")
	if res.Date == "" {
		res.Date = res.Today
	} else if !gameday.Valid(res.Date) {
		http.Error(w, "Bad date, use YYYY-MM-DD.", http.StatusBadRequest)
		return
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 0 {
			http.Error(w, "Bad page.", http.StatusBadRequest)
			return
		}
		res.Page = page
	}

	rows, total, err := svc.store.WordleLeaderboard(res.Date, leaderboardPageSize, res.Page*leaderboardPageSize)
	if err != nil {
		slog.Error("getting leaderboard", "err", err)
		http.Error(w, "Database error.", http.StatusInternalServerError)
		return
	}
	res.Rows = rows
	res.Total = total
	writeJSON(w, res)
}
