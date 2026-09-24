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

// A player with one character
func newCharacter(t *testing.T, s *Store, name string) Character {
	t.Helper()
	s.InsertPlayer(name, "h")
	p, _, _ := s.PlayerByUsername(name)
	c, _, err := s.InsertCharacter(p.ID, 0, name+"-char", "rogue")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestWordleResultsAndLeaderboard(t *testing.T) {
	s := open(t)
	chars := map[string]Character{}
	for _, name := range []string{"a", "b", "c", "d"} {
		chars[name] = newCharacter(t, s, name)
	}
	result := func(name, date string, win bool, secs float64, guesses int) WordleResult {
		c := chars[name]
		return WordleResult{Date: date, Win: win, Seconds: secs, GuessCount: guesses, PlayerID: c.PlayerID, CharacterID: c.ID}
	}
	const day = "2026-09-23"
	results := []WordleResult{
		result("a", day, true, 90, 3),
		result("b", day, true, 30, 3), // same guesses, faster
		result("c", day, true, 10, 4),
		result("d", day, false, 5, 5), // losses aren't ranked
		result("a", "2026-09-22", true, 1, 1),
	}
	for _, r := range results {
		if err := s.InsertWordle(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.InsertWordle(results[0]); err == nil {
		t.Fatal("second result for the same player and day was accepted")
	}

	if played, _ := s.PlayedWordleOn(chars["d"].PlayerID, day); !played {
		t.Fatal("d played (and lost) but PlayedWordleOn says no")
	}
	if played, _ := s.PlayedWordleOn(chars["d"].PlayerID, "2026-09-22"); played {
		t.Fatal("d didn't play on the 22nd")
	}

	rows, total, err := s.WordleLeaderboard(day, 2, 0)
	if err != nil || total != 3 || len(rows) != 2 {
		t.Fatalf("page 1: %+v total=%d err=%v", rows, total, err)
	}
	if rows[0].Uname != "b" || rows[0].Place != 1 || rows[1].Uname != "a" || rows[1].Place != 2 {
		t.Fatalf("wrong order, want fewest guesses then fastest: %+v", rows)
	}
	if r := rows[0]; r.Char != "b-char" || r.Class != "rogue" || r.CharacterID != chars["b"].ID {
		t.Fatalf("rows should say which character played: %+v", r)
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
