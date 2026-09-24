package ranked

import (
	"math"
	"testing"

	"nytrpg/internal/protocol"
)

func TestLadder(t *testing.T) {
	if len(Ladder) != 16 {
		t.Fatalf("%d ranks, want 5 tiers x 3 divisions + Master", len(Ladder))
	}
	for elo, want := range map[int]string{
		0: "Iron 3", 499: "Iron 3", 500: "Iron 2", 699: "Iron 1", 700: "Bronze 3",
		999: "Bronze 1", StartElo: "Silver 3", 1299: "Silver 1", 1300: "Gold 3",
		1600: "Diamond 3", 1899: "Diamond 1", 1900: "Master", 5000: "Master",
	} {
		if got := TierFor(elo); got.Name != want {
			t.Errorf("TierFor(%d) = %s, want %s", elo, got.Name, want)
		}
	}
	if m := TierFor(2000); m.ID != "master" || m.Family != "master" {
		t.Fatalf("master tier %+v", m)
	}
	if s := TierFor(1150); s.ID != "silver-2" || s.Family != "silver" {
		t.Fatalf("silver 2 tier %+v", s)
	}
}

func TestExpected(t *testing.T) {
	if e := Expected(1000, 1000); e != 0.5 {
		t.Fatalf("equal players: %v", e)
	}
	a, b := Expected(1400, 1000), Expected(1000, 1400)
	if math.Abs(a+b-1) > 1e-9 || a < 0.9 {
		t.Fatalf("400 points apart: %v and %v", a, b)
	}
}

func TestMarginMultiplier(t *testing.T) {
	close := Margin{WinnerGuesses: 5, LoserGuesses: 4, LoserBest: 0.9, WinnerSeconds: 300}
	crushing := Margin{WinnerGuesses: 2, LoserGuesses: 5, LoserOut: true, LoserBest: 0, WinnerSeconds: 20}
	if m := close.Multiplier(); m < MinMargin || m > 1.1 {
		t.Fatalf("a narrow win should barely count extra: %v", m)
	}
	if m := crushing.Multiplier(); m <= 1.6 || m > MaxMargin {
		t.Fatalf("a crushing win should be near the max: %v", m)
	}
	if m := (Margin{Forfeit: true}).Multiplier(); m != MaxMargin {
		t.Fatalf("forfeit: %v", m)
	}
	// Each factor on its own pushes it up
	base := Margin{WinnerGuesses: 4, LoserGuesses: 3, LoserBest: 0.6, WinnerSeconds: 120}
	for name, better := range map[string]Margin{
		"fewer winner guesses": {WinnerGuesses: 2, LoserGuesses: 3, LoserBest: 0.6, WinnerSeconds: 120},
		"loser further off":    {WinnerGuesses: 4, LoserGuesses: 3, LoserBest: 0.2, WinnerSeconds: 120},
		"faster solve":         {WinnerGuesses: 4, LoserGuesses: 3, LoserBest: 0.6, WinnerSeconds: 30},
	} {
		if better.Multiplier() <= base.Multiplier() {
			t.Errorf("%s should raise the margin: %v <= %v", name, better.Multiplier(), base.Multiplier())
		}
	}
}

func TestCloseness(t *testing.T) {
	g, y, x := protocol.Green, protocol.Yellow, protocol.Grey
	if c := Closeness([]protocol.WordleColor{g, g, y, x, x}); c != 0.5 {
		t.Fatalf("2 greens and a yellow: %v", c)
	}
	if Closeness(nil) != 0 || Closeness([]protocol.WordleColor{g, g, g, g, g}) != 1 {
		t.Fatal("closeness range")
	}
}

func established(elo int) Rating {
	return Rating{Elo: elo, Peak: elo, Games: 50}
}

func TestSettle(t *testing.T) {
	m := Margin{WinnerGuesses: 3, LoserGuesses: 3, LoserBest: 0.4, WinnerSeconds: 90}

	res := Settle(established(1000), established(1000), AWins, m)
	if res.DeltaA <= 0 || res.DeltaA != -res.DeltaB {
		t.Fatalf("zero-sum between established players: %+d %+d", res.DeltaA, res.DeltaB)
	}
	if res.A.Elo != 1000+res.DeltaA || res.A.Wins != 1 || res.B.Losses != 1 || res.A.Games != 51 {
		t.Fatalf("ratings %+v %+v", res.A, res.B)
	}
	if res.A.Peak != res.A.Elo || res.B.Peak != 1000 {
		t.Fatalf("peaks %d %d", res.A.Peak, res.B.Peak)
	}

	// Beating someone much stronger is worth more than beating someone weaker
	upset := Settle(established(1000), established(1400), AWins, m)
	expected := Settle(established(1400), established(1000), AWins, m)
	if upset.DeltaA <= expected.DeltaA {
		t.Fatalf("upset +%d should beat expected win +%d", upset.DeltaA, expected.DeltaA)
	}
	// Even a sure win moves at least a point
	sure := Settle(established(2800), established(100), AWins, Margin{WinnerGuesses: 5, LoserGuesses: 1, LoserBest: 1, WinnerSeconds: 999})
	if sure.DeltaA < 1 || sure.DeltaB > -1 {
		t.Fatalf("a decisive result should always move: %+d %+d", sure.DeltaA, sure.DeltaB)
	}

	// B wins mirrors A wins
	bw := Settle(established(1000), established(1000), BWins, m)
	if bw.DeltaB != res.DeltaA || bw.DeltaA != res.DeltaB {
		t.Fatalf("B winning: %+d %+d", bw.DeltaA, bw.DeltaB)
	}

	// Drawing someone equal changes nothing but the game count
	d := Settle(established(1200), established(1200), Draw, m)
	if d.DeltaA != 0 || d.DeltaB != 0 || d.A.Draws != 1 || d.Multiplier != 1 {
		t.Fatalf("draw: %+v", d)
	}
}

func TestProvisionalAndFloor(t *testing.T) {
	m := Margin{WinnerGuesses: 3, LoserGuesses: 3, LoserBest: 0.4, WinnerSeconds: 90}
	fresh := Settle(NewRating(), established(1000), AWins, m)
	if fresh.DeltaA <= -fresh.DeltaB {
		t.Fatalf("a new character should move more: %+d vs %+d", fresh.DeltaA, fresh.DeltaB)
	}

	broke := Settle(established(5), established(5), BWins, Margin{Forfeit: true})
	if broke.A.Elo != 0 || broke.DeltaA != -5 {
		t.Fatalf("elo floors at 0: %+v %+d", broke.A, broke.DeltaA)
	}

	forfeit := Settle(established(1000), established(1000), AWins, Margin{Forfeit: true})
	if forfeit.Multiplier != MaxMargin {
		t.Fatalf("forfeit margin %v", forfeit.Multiplier)
	}
}

func TestStakesMatchSettle(t *testing.T) {
	me, them := established(1100), established(1350)
	s := Stakes(me, them)
	if !(s.WinMin > 0 && s.WinMax > s.WinMin && s.LoseMin < 0 && s.LoseMax < s.LoseMin) {
		t.Fatalf("stakes %+v", s)
	}
	// The biggest win and loss are what a forfeit settles to
	if w := Settle(me, them, AWins, Margin{Forfeit: true}); w.DeltaA != s.WinMax {
		t.Fatalf("max win %d, settle %d", s.WinMax, w.DeltaA)
	}
	if l := Settle(me, them, BWins, Margin{Forfeit: true}); l.DeltaA != s.LoseMax {
		t.Fatalf("max loss %d, settle %d", s.LoseMax, l.DeltaA)
	}
	// Nothing to lose below 0
	if s := Stakes(established(3), established(3)); s.LoseMax != -3 || s.LoseMin != -3 {
		t.Fatalf("stakes at 3 elo: %+v", s)
	}
}
