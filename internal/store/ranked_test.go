package store

import "testing"

func twoCharacters(t *testing.T, s *Store) (Character, Character) {
	t.Helper()
	for _, name := range []string{"alice", "bob"} {
		if _, err := s.InsertPlayer(name, "hash"); err != nil {
			t.Fatal(err)
		}
	}
	a, _, _ := s.PlayerByUsername("alice")
	b, _, _ := s.PlayerByUsername("bob")
	ca, _, err := s.InsertCharacter(a.ID, 0, "Arthur", "knight")
	if err != nil {
		t.Fatal(err)
	}
	cb, _, err := s.InsertCharacter(b.ID, 0, "Merlin", "wizard")
	if err != nil {
		t.Fatal(err)
	}
	return ca, cb
}

func TestRankedDuelsAndRatings(t *testing.T) {
	s := open(t)
	a, b := twoCharacters(t, s)

	if _, found, err := s.Rating(a.ID); err != nil || found {
		t.Fatalf("a character that never played ranked has no rating row: %v %v", found, err)
	}

	// Arthur beats Merlin, then they draw
	err := s.ApplyRankedDuel(
		RankedDuel{PlayedAt: 100, A: a.ID, B: b.ID, Winner: a.ID, ABefore: 1000, AAfter: 1020, BBefore: 1000, BAfter: 980, AGuesses: 3, BGuesses: 4, Seconds: 40},
		Rating{CharacterID: a.ID, Elo: 1020, Peak: 1020, Games: 1, Wins: 1},
		Rating{CharacterID: b.ID, Elo: 980, Peak: 1000, Games: 1, Losses: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	err = s.ApplyRankedDuel(
		RankedDuel{PlayedAt: 200, A: b.ID, B: a.ID, ABefore: 980, AAfter: 982, BBefore: 1020, BAfter: 1018, AGuesses: 5, BGuesses: 5, Seconds: 90},
		Rating{CharacterID: b.ID, Elo: 982, Peak: 1000, Games: 2, Losses: 1, Draws: 1},
		Rating{CharacterID: a.ID, Elo: 1018, Peak: 1020, Games: 2, Wins: 1, Draws: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	r, found, err := s.Rating(a.ID)
	if err != nil || !found || r.Elo != 1018 || r.Peak != 1020 || r.Games != 2 || r.Wins != 1 || r.Draws != 1 {
		t.Fatalf("Arthur's rating %+v %v %v", r, found, err)
	}

	recent, err := s.RecentRankedDuels(a.ID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("%d duels", len(recent))
	}
	draw, win := recent[0], recent[1]
	if !draw.Draw || draw.Change != -2 || draw.Opponent != "Merlin" || draw.OpponentClass != "wizard" || draw.PlayedAt != 200 {
		t.Fatalf("latest, the draw, from Arthur's side: %+v", draw)
	}
	if !win.Won || win.Change != 20 || win.Opponent != "Merlin" {
		t.Fatalf("the win: %+v", win)
	}
	merlin, _ := s.RecentRankedDuels(b.ID, 1)
	if len(merlin) != 1 || merlin[0].Change != 2 || merlin[0].Opponent != "Arthur" {
		t.Fatalf("limit and Merlin's side: %+v", merlin)
	}

	// A deleted character keeps its history
	if _, err := s.DeleteCharacter(b.PlayerID, b.ID); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.RecentRankedDuels(a.ID, 5); len(again) != 2 || again[0].Opponent != "Merlin" {
		t.Fatalf("history after deleting the opponent: %+v", again)
	}
}
