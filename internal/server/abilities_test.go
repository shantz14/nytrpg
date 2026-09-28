package server_test

import (
	"reflect"
	"testing"
	"time"

	"nytrpg/internal/classes"
	"nytrpg/internal/protocol"
	"nytrpg/internal/testkit"
)

// The latest DuelState c got within the timeout that passes ok
func expectState(t *testing.T, c *testkit.Client, ok func(protocol.DuelState) bool) protocol.DuelState {
	t.Helper()
	deadline := time.Now().Add(testkit.Timeout)
	for time.Now().Before(deadline) {
		if s := testkit.Expect[protocol.DuelState](t, c, protocol.ServerDuelState); ok(s) {
			return s
		}
	}
	t.Fatal("no matching duel state")
	return protocol.DuelState{}
}

func TestAbilitiesOverTheWire(t *testing.T) {
	t.Parallel()
	_, cs := duelistsAs(t, []string{"knight", "wizard"}, "knight", "wizard")
	knight, wizard := cs[0], cs[1]
	accept(t, knight, wizard, challenge(t, knight, wizard))
	expectState(t, knight, func(s protocol.DuelState) bool { return s.You.Energy == 0 && s.You.Rows == 5 })

	// Letters found pay energy: TRACE on CRANE has R, A, E green and C yellow
	wizard.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "trace"})
	testkit.Expect[protocol.WordleRes](t, wizard, protocol.ServerDuelGuess)
	expectState(t, knight, func(s protocol.DuelState) bool { return s.Them.Energy == 7 })
	// SLATE: A and E green
	knight.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "slate"})
	testkit.Expect[protocol.WordleRes](t, knight, protocol.ServerDuelGuess)
	expectState(t, knight, func(s protocol.DuelState) bool { return s.You.Energy == 4 })
	// The knight's new greens stunned the wizard (Aggressive)
	if ev := testkit.Expect[protocol.DuelCast](t, wizard, protocol.ServerDuelCast); ev.Ability != classes.Aggressive || ev.ByYou {
		t.Fatalf("aggressive %+v", ev)
	}

	// Slash the wizard's R: both see which tile, neither sees a letter
	knight.Send(t, protocol.ClientDuelCast, protocol.DuelCastReq{Slot: 0, Row: 0, Col: 1})
	for _, c := range []*testkit.Client{knight, wizard} {
		ev := testkit.Expect[protocol.DuelCast](t, c, protocol.ServerDuelCast)
		if ev.Ability != classes.Slash || ev.ByYou != (c == knight) || ev.Row != 0 || ev.Col != 1 {
			t.Fatalf("slash seen as %+v", ev)
		}
	}
	expectState(t, knight, func(s protocol.DuelState) bool { return s.You.Energy == 2 })

	// Scry: only the wizard hears the answer
	wizard.Send(t, protocol.ClientDuelCast, protocol.DuelCastReq{Slot: 0, Letter: "n"})
	if s := testkit.Expect[protocol.DuelScry](t, wizard, protocol.ServerDuelScry); s.Letter != "N" || !s.InWord {
		t.Fatalf("scry %+v", s)
	}
	if ev := testkit.Expect[protocol.DuelCast](t, knight, protocol.ServerDuelCast); ev.Ability != classes.Scry || ev.ByYou {
		t.Fatalf("knight saw %+v", ev)
	}
	knight.ExpectNone(t, protocol.ServerDuelScry, 200*time.Millisecond)
}

func TestRogueAndClericOverTheWire(t *testing.T) {
	t.Parallel()
	_, cs := duelistsAs(t, []string{"rogue", "cleric"}, "rogue", "cleric")
	rogue, cleric := cs[0], cs[1]
	accept(t, rogue, cleric, challenge(t, rogue, cleric))

	// TRACE: 7 energy, enough for Under Their Nose
	rogue.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "trace"})
	testkit.Expect[protocol.WordleRes](t, rogue, protocol.ServerDuelGuess)
	expectState(t, rogue, func(s protocol.DuelState) bool { return s.You.Energy == 7 })
	fake := []protocol.WordleColor{protocol.Green, protocol.Yellow, protocol.Grey, protocol.Grey, protocol.Green}
	rogue.Send(t, protocol.ClientDuelCast, protocol.DuelCastReq{Slot: 3, Colors: fake})
	if ev := testkit.Expect[protocol.DuelCast](t, rogue, protocol.ServerDuelCast); ev.Ability != classes.UnderTheirNose || !ev.ByYou {
		t.Fatalf("rogue told %+v", ev)
	}
	// The cleric heard about TRACE's Sneaky, and nothing since
	if ev := testkit.Expect[protocol.DuelCast](t, cleric, protocol.ServerDuelCast); ev.Ability != classes.Sneaky {
		t.Fatalf("cleric told %+v", ev)
	}
	cleric.ExpectNone(t, protocol.ServerDuelCast, 200*time.Millisecond)

	// The cleric's PILOT shows them the fake colors, the rogue sees the real
	// ones, and the cleric is never told
	cleric.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "pilot"})
	if res := testkit.Expect[protocol.WordleRes](t, cleric, protocol.ServerDuelGuess); !reflect.DeepEqual(res.Colors, fake) {
		t.Fatalf("cleric saw %v", res.Colors)
	}
	grey := make([]protocol.WordleColor, 5)
	if og := testkit.Expect[protocol.DuelOpponentGuess](t, rogue, protocol.ServerDuelOpponentGuess); !reflect.DeepEqual(og.Colors, grey) {
		t.Fatalf("rogue saw %v", og.Colors)
	}

	// SLATE: 4 energy, then a Minor Prayer, answered (by the test server's
	// canned gods, never the network)
	cleric.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "slate"})
	testkit.Expect[protocol.WordleRes](t, cleric, protocol.ServerDuelGuess)
	expectState(t, cleric, func(s protocol.DuelState) bool { return s.You.Energy == 4 })
	cleric.Send(t, protocol.ClientDuelCast, protocol.DuelCastReq{Slot: 0})
	if p := testkit.Expect[protocol.DuelPrayer](t, cleric, protocol.ServerDuelPrayer); !p.Pending {
		t.Fatalf("pending %+v", p)
	}
	if p := testkit.Expect[protocol.DuelPrayer](t, cleric, protocol.ServerDuelPrayer); p.Failed || p.God == "" || p.Text == "" {
		t.Fatalf("answer %+v", p)
	}
	rogue.ExpectNone(t, protocol.ServerDuelPrayer, 200*time.Millisecond)
}

func TestCastsAreRateLimited(t *testing.T) {
	t.Parallel()
	_, cs := duelistsAs(t, []string{"knight", "rogue"}, "knight", "rogue")
	knight, rogue := cs[0], cs[1]
	accept(t, knight, rogue, challenge(t, knight, rogue))
	testkit.Expect[protocol.DuelState](t, knight, protocol.ServerDuelState)

	// Cripple without the energy: each cast that gets through is answered
	// with the knight's real state
	for range 30 {
		knight.Send(t, protocol.ClientDuelCast, protocol.DuelCastReq{Slot: 4})
	}
	if n := knight.Count(protocol.ServerDuelState, 500*time.Millisecond); n < 1 || n > 7 {
		t.Fatalf("%d of 30 casts got through", n)
	}
}
