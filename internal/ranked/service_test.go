package ranked

import (
	"path/filepath"
	"testing"
	"time"

	"nytrpg/internal/protocol"
	"nytrpg/internal/store"
)

func TestServiceLoadSeesRecordedDuels(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.InsertPlayer("alice", "h")
	st.InsertPlayer("bob", "h")
	pa, _, _ := st.PlayerByUsername("alice")
	pb, _, _ := st.PlayerByUsername("bob")
	a, _, _ := st.InsertCharacter(pa.ID, 0, "Arthur", "knight")
	b, _, _ := st.InsertCharacter(pb.ID, 0, "Merlin", "wizard")

	svc := NewService(st)
	defer svc.Close()
	if r := svc.Load(a.ID); r != NewRating() {
		t.Fatalf("new character: %+v", r)
	}

	// Many duels queued, then a load straight away: it must see all of them
	ra, rb := svc.Load(a.ID), svc.Load(b.ID)
	for i := 0; i < 20; i++ {
		res := Settle(ra, rb, AWins, Margin{WinnerGuesses: 3, LoserGuesses: 3, LoserBest: 0.5, WinnerSeconds: 60})
		svc.Record(Match{PlayedAt: time.Unix(int64(i), 0), A: a.ID, B: b.ID, Outcome: AWins, Result: res, AGuesses: 3, BGuesses: 3, Seconds: 60})
		ra, rb = res.A, res.B
	}
	if got := svc.Load(a.ID); got != ra {
		t.Fatalf("loaded %+v, want %+v", got, ra)
	}
	if r := svc.Load(b.ID); r != rb {
		t.Fatalf("loaded %+v, want %+v", r, rb)
	}
	recent := svc.Recent(b.ID)
	if len(recent) != RecentDuels || recent[0].Outcome != protocol.DuelLose || recent[0].Opponent != "Arthur" || recent[0].Change >= 0 {
		t.Fatalf("recent %+v", recent)
	}
	if r := svc.Recent(a.ID); r[0].Outcome != protocol.DuelWin || r[0].OpponentClass != "wizard" {
		t.Fatalf("recent from the winner's side %+v", r)
	}
}

func TestServiceCloseSavesWhatsQueued(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.InsertPlayer("alice", "h")
	st.InsertPlayer("bob", "h")
	pa, _, _ := st.PlayerByUsername("alice")
	pb, _, _ := st.PlayerByUsername("bob")
	a, _, _ := st.InsertCharacter(pa.ID, 0, "Arthur", "knight")
	b, _, _ := st.InsertCharacter(pb.ID, 0, "Merlin", "wizard")

	svc := NewService(st)
	res := Settle(NewRating(), NewRating(), BWins, Margin{Forfeit: true})
	svc.Record(Match{PlayedAt: time.Unix(1, 0), A: a.ID, B: b.ID, Outcome: BWins, Result: res})
	svc.Close()
	if r, found, _ := st.Rating(b.ID); !found || r.Elo != res.B.Elo {
		t.Fatalf("shutdown lost a duel: %+v %v", r, found)
	}
}
