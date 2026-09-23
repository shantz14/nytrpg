package store

import (
	"path/filepath"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrationsRunOnceAndSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertPlayer("alice", "hash"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer s.Close()
	var version int
	if err := s.db.QueryRow("SELECT version FROM schema_version").Scan(&version); err != nil || version != len(migrations) {
		t.Fatalf("schema version %d (%v), want %d", version, err, len(migrations))
	}
	if _, found, _ := s.PlayerByUsername("alice"); !found {
		t.Fatal("data lost on reopen")
	}
}

func TestPlayers(t *testing.T) {
	s := open(t)
	if taken, err := s.InsertPlayer("alice", "hash1"); taken || err != nil {
		t.Fatalf("first insert: taken=%v err=%v", taken, err)
	}
	if taken, err := s.InsertPlayer("alice", "hash2"); !taken || err != nil {
		t.Fatalf("duplicate should be taken: taken=%v err=%v", taken, err)
	}

	p, hash, found, err := s.PlayerAuth("alice")
	if !found || err != nil || hash != "hash1" || p.Username != "alice" || p.ID == 0 {
		t.Fatalf("PlayerAuth: %+v %q %v %v", p, hash, found, err)
	}
	if _, _, found, err := s.PlayerAuth("nobody"); found || err != nil {
		t.Fatalf("unknown player: found=%v err=%v", found, err)
	}
	if byName, found, err := s.PlayerByUsername("alice"); !found || err != nil || byName.ID != p.ID {
		t.Fatalf("PlayerByUsername: %+v %v %v", byName, found, err)
	}
}

func TestWordleResultsAndLeaderboard(t *testing.T) {
	s := open(t)
	ids := map[string]int{}
	for _, name := range []string{"a", "b", "c", "d"} {
		s.InsertPlayer(name, "h")
		p, _, _ := s.PlayerByUsername(name)
		ids[name] = p.ID
	}
	const day = "2026-09-23"
	results := []WordleResult{
		{Date: day, Win: true, Seconds: 90, GuessCount: 3, PlayerID: ids["a"]},
		{Date: day, Win: true, Seconds: 30, GuessCount: 3, PlayerID: ids["b"]}, // same guesses, faster
		{Date: day, Win: true, Seconds: 10, GuessCount: 4, PlayerID: ids["c"]},
		{Date: day, Win: false, Seconds: 5, GuessCount: 5, PlayerID: ids["d"]}, // losses aren't ranked
		{Date: "2026-09-22", Win: true, Seconds: 1, GuessCount: 1, PlayerID: ids["a"]},
	}
	for _, r := range results {
		if err := s.InsertWordle(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.InsertWordle(results[0]); err == nil {
		t.Fatal("second result for the same player and day was accepted")
	}

	if played, _ := s.PlayedWordleOn(ids["d"], day); !played {
		t.Fatal("d played (and lost) but PlayedWordleOn says no")
	}
	if played, _ := s.PlayedWordleOn(ids["d"], "2026-09-22"); played {
		t.Fatal("d didn't play on the 22nd")
	}

	rows, total, err := s.WordleLeaderboard(day, 2, 0)
	if err != nil || total != 3 || len(rows) != 2 {
		t.Fatalf("page 1: %+v total=%d err=%v", rows, total, err)
	}
	if rows[0].Uname != "b" || rows[0].Place != 1 || rows[1].Uname != "a" || rows[1].Place != 2 {
		t.Fatalf("wrong order, want fewest guesses then fastest: %+v", rows)
	}
	rows, _, _ = s.WordleLeaderboard(day, 2, 2)
	if len(rows) != 1 || rows[0].Uname != "c" || rows[0].Place != 3 {
		t.Fatalf("page 2 should continue numbering: %+v", rows)
	}
	rows, total, _ = s.WordleLeaderboard("2020-01-01", 10, 0)
	if total != 0 || rows == nil || len(rows) != 0 {
		t.Fatalf("empty day should be an empty list, not nil: %+v %d", rows, total)
	}
}
