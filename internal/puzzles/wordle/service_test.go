package wordle

import (
	"path/filepath"
	"testing"

	"nytrpg/internal/gameday"
	"nytrpg/internal/netconn"
	"nytrpg/internal/store"
)

// A service and a session for a player with two characters, playing the first
func newTestService(t *testing.T, near bool) (*Service, *store.Store, *netconn.Session, store.Character) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.InsertPlayer("p", "h")
	p, _, _ := st.PlayerByUsername("p")
	c, _, err := st.InsertCharacter(p.ID, 0, "c", "knight")
	if err != nil {
		t.Fatal(err)
	}
	alt, _, err := st.InsertCharacter(p.ID, 1, "alt", "rogue")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, func(*netconn.Session) bool { return near })
	return svc, st, &netconn.Session{PlayerID: p.ID, CharacterID: c.ID}, alt
}

func TestStartTooFar(t *testing.T) {
	svc, _, s, _ := newTestService(t, false)
	if r := svc.start(s); !r.TooFar || r.Played {
		t.Fatalf("want tooFar: %+v", r)
	}
	// No session was started, so guesses are refused
	if res := svc.guess(s, "CRANE"); res.Valid {
		t.Fatal("guess accepted without a session")
	}
}

func TestFinishedGameIsSavedOnceAndBlocksReplay(t *testing.T) {
	svc, st, s, alt := newTestService(t, true)
	if r := svc.start(s); r.Played || r.TooFar {
		t.Fatalf("fresh start: %+v", r)
	}

	answer := svc.words.For(gameday.Today())
	res := svc.guess(s, answer)
	if !res.Valid || res.Solution != answer {
		t.Fatalf("winning guess: %+v", res)
	}
	if played, _ := st.PlayedWordleOn(s.PlayerID, gameday.Today()); !played {
		t.Fatal("result wasn't saved")
	}
	rows, total, _ := st.WordleLeaderboard(gameday.Today(), 10, 0)
	if total != 1 || rows[0].Guesses != 1 || rows[0].CharacterID != s.CharacterID {
		t.Fatalf("leaderboard should credit the character who played: %+v", rows)
	}

	if r := svc.start(s); !r.Played {
		t.Fatalf("start after finishing: %+v", r)
	}
	// Even a stale session can't play again once the result is in the db
	svc.sessions.sessions[s.PlayerID].done = false
	if svc.guess(s, answer).Valid {
		t.Fatal("played twice in a day")
	}

	// The daily game is per account: another character can't play it again
	other := &netconn.Session{PlayerID: s.PlayerID, CharacterID: alt.ID}
	if r := svc.start(other); !r.Played {
		t.Fatalf("second character should see today's game as played: %+v", r)
	}
	if svc.guess(other, answer).Valid {
		t.Fatal("second character played the same day")
	}
}

// Switching characters mid-game continues the same game, and whoever finishes
// it gets the result
func TestSwitchingCharactersContinuesTheGame(t *testing.T) {
	svc, st, s, alt := newTestService(t, true)
	svc.start(s)
	wrong := "CRANE"
	if svc.words.For(gameday.Today()) == wrong {
		wrong = "SLATE"
	}
	if res := svc.guess(s, wrong); !res.Valid {
		t.Fatalf("first guess: %+v", res)
	}

	other := &netconn.Session{PlayerID: s.PlayerID, CharacterID: alt.ID}
	r := svc.start(other)
	if r.Played || len(r.Guesses) != 1 || r.Guesses[0] != wrong {
		t.Fatalf("the other character should resume the game: %+v", r)
	}
	answer := svc.words.For(gameday.Today())
	if res := svc.guess(other, answer); res.Solution != answer {
		t.Fatalf("finishing guess: %+v", res)
	}
	rows, _, _ := st.WordleLeaderboard(gameday.Today(), 10, 0)
	if len(rows) != 1 || rows[0].CharacterID != alt.ID || rows[0].Guesses != 2 {
		t.Fatalf("result should go to the character who finished: %+v", rows)
	}
}
