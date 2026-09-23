package wordle

import (
	"path/filepath"
	"testing"

	"nytrpg/internal/gameday"
	"nytrpg/internal/netconn"
	"nytrpg/internal/store"
)

func newTestService(t *testing.T, near bool) (*Service, *store.Store, int) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.InsertPlayer("p", "h")
	p, _, _ := st.PlayerByUsername("p")
	svc := NewService(st, func(*netconn.Session) bool { return near })
	return svc, st, p.ID
}

func TestStartTooFar(t *testing.T) {
	svc, _, id := newTestService(t, false)
	if r := svc.start(&netconn.Session{PlayerID: id}); !r.TooFar || r.Played {
		t.Fatalf("want tooFar: %+v", r)
	}
	// No session was started, so guesses are refused
	if res := svc.guess(id, "CRANE"); res.Valid {
		t.Fatal("guess accepted without a session")
	}
}

func TestFinishedGameIsSavedOnceAndBlocksReplay(t *testing.T) {
	svc, st, id := newTestService(t, true)
	s := &netconn.Session{PlayerID: id}
	if r := svc.start(s); r.Played || r.TooFar {
		t.Fatalf("fresh start: %+v", r)
	}

	answer := svc.words.For(gameday.Today())
	res := svc.guess(id, answer)
	if !res.Valid || res.Solution != answer {
		t.Fatalf("winning guess: %+v", res)
	}
	if played, _ := st.PlayedWordleOn(id, gameday.Today()); !played {
		t.Fatal("result wasn't saved")
	}
	rows, total, _ := st.WordleLeaderboard(gameday.Today(), 10, 0)
	if total != 1 || rows[0].Guesses != 1 {
		t.Fatalf("leaderboard: %+v", rows)
	}

	if r := svc.start(s); !r.Played {
		t.Fatalf("start after finishing: %+v", r)
	}
	// Even a stale session can't play again once the result is in the db
	svc.sessions.sessions[id].done = false
	if svc.guess(id, answer).Valid {
		t.Fatal("played twice in a day")
	}
}
