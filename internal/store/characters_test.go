package store

import (
	"path/filepath"
	"testing"
)

func TestCharacters(t *testing.T) {
	s := open(t)
	s.InsertPlayer("alice", "h")
	s.InsertPlayer("bob", "h")
	alice, _, _ := s.PlayerByUsername("alice")
	bob, _, _ := s.PlayerByUsername("bob")

	knight, taken, err := s.InsertCharacter(alice.ID, 0, "Arthur", "knight")
	if taken || err != nil || knight.ID == 0 {
		t.Fatalf("insert: %+v taken=%v err=%v", knight, taken, err)
	}
	if _, taken, err := s.InsertCharacter(alice.ID, 0, "Lancelot", "knight"); !taken || err != nil {
		t.Fatalf("slot 0 is used, should be taken: taken=%v err=%v", taken, err)
	}
	// Names aren't unique, and slots are per player
	if _, taken, err := s.InsertCharacter(bob.ID, 0, "Arthur", "wizard"); taken || err != nil {
		t.Fatalf("bob's slot 0: taken=%v err=%v", taken, err)
	}
	if _, _, err := s.InsertCharacter(alice.ID, CharacterSlots, "Nope", "knight"); err == nil {
		t.Fatal("slot out of range should violate the slot check")
	}
	s.InsertCharacter(alice.ID, 2, "Morgana", "wizard")

	chars, err := s.Characters(alice.ID)
	if err != nil || len(chars) != 2 || chars[0].Name != "Arthur" || chars[1].Slot != 2 {
		t.Fatalf("alice's characters by slot: %+v %v", chars, err)
	}

	if found, err := s.DeleteCharacter(bob.ID, knight.ID); found || err != nil {
		t.Fatalf("bob deleted alice's character: found=%v err=%v", found, err)
	}
	if found, err := s.DeleteCharacter(alice.ID, knight.ID); !found || err != nil {
		t.Fatalf("delete: found=%v err=%v", found, err)
	}
	if found, _ := s.DeleteCharacter(alice.ID, knight.ID); found {
		t.Fatal("deleted twice")
	}
	if c, found, _ := s.CharacterByID(knight.ID); !found || !c.Deleted {
		t.Fatalf("deleted characters are kept, marked deleted: %+v %v", c, found)
	}
	if chars, _ := s.Characters(alice.ID); len(chars) != 1 {
		t.Fatalf("deleted character still listed: %+v", chars)
	}
	if _, taken, err := s.InsertCharacter(alice.ID, 0, "Lancelot", "knight"); taken || err != nil {
		t.Fatalf("deleting should free the slot: taken=%v err=%v", taken, err)
	}
	if _, found, err := s.CharacterByID(9999); found || err != nil {
		t.Fatalf("unknown id: found=%v err=%v", found, err)
	}
}

// The daily wordle is once per account, whichever character plays it
func TestWordleIsPerAccount(t *testing.T) {
	s := open(t)
	s.InsertPlayer("alice", "h")
	alice, _, _ := s.PlayerByUsername("alice")
	a, _, _ := s.InsertCharacter(alice.ID, 0, "A", "knight")
	b, _, _ := s.InsertCharacter(alice.ID, 1, "B", "cleric")
	const day = "2026-09-23"
	if err := s.InsertWordle(WordleResult{Date: day, Win: true, GuessCount: 2, PlayerID: alice.ID, CharacterID: a.ID}); err != nil {
		t.Fatal(err)
	}
	if played, _ := s.PlayedWordleOn(alice.ID, day); !played {
		t.Fatal("alice played today")
	}
	if err := s.InsertWordle(WordleResult{Date: day, Win: true, GuessCount: 4, PlayerID: alice.ID, CharacterID: b.ID}); err == nil {
		t.Fatal("a second character played the same day")
	}
	// A deleted character's result stays on the leaderboard, and deleting it
	// doesn't let the account play again
	s.DeleteCharacter(alice.ID, a.ID)
	if played, _ := s.PlayedWordleOn(alice.ID, day); !played {
		t.Fatal("deleting the character that played reset the day")
	}
	rows, total, _ := s.WordleLeaderboard(day, 10, 0)
	if total != 1 || rows[0].Char != "A" || rows[0].Uname != "alice" {
		t.Fatalf("leaderboard: %+v", rows)
	}
}

// Wordle results from before characters existed move to a knight named after the player
func TestMigrationGivesOldPlayersACharacter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Roll back to the schema before characters and fill it the old way
	for _, q := range []string{
		"DROP TABLE Wordle", "DROP TABLE Character", "DROP TABLE Player",
		migrations[0],
		"UPDATE schema_version SET version = 1",
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	s.InsertPlayer("veteran", "h")
	s.InsertPlayer("newbie", "h")
	vet, _, _ := s.PlayerByUsername("veteran")
	if _, err := s.db.Exec("INSERT INTO Wordle (date, win, seconds, guessCount, player_id) VALUES ('2026-09-01', 1, 42, 3, ?)", vet.ID); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatalf("migrating: %v", err)
	}
	defer s.Close()
	chars, _ := s.Characters(vet.ID)
	if len(chars) != 1 || chars[0].Name != "veteran" || chars[0].Class != "knight" || chars[0].Slot != 0 {
		t.Fatalf("veteran should get a knight: %+v", chars)
	}
	if played, _ := s.PlayedWordleOn(vet.ID, "2026-09-01"); !played {
		t.Fatal("old result didn't move to the new character")
	}
	rows, _, _ := s.WordleLeaderboard("2026-09-01", 10, 0)
	if len(rows) != 1 || rows[0].Uname != "veteran" || rows[0].Time != 42 {
		t.Fatalf("leaderboard lost the old result: %+v", rows)
	}
	newbie, _, _ := s.PlayerByUsername("newbie")
	if chars, _ := s.Characters(newbie.ID); len(chars) != 0 {
		t.Fatalf("players who never played start with no characters: %+v", chars)
	}
}
