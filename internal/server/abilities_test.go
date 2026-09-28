package server_test

import (
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
