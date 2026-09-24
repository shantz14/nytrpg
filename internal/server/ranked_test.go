package server_test

import (
	"testing"
	"time"

	"nytrpg/internal/protocol"
	"nytrpg/internal/ranked"
	"nytrpg/internal/testkit"
)

func TestRankedDuelIsSavedAndSurvivesReconnect(t *testing.T) {
	t.Parallel()
	ts, cs := duelists(t, "a", "b")
	a, b := cs[0], cs[1]
	if a.Welcome.Elo != ranked.StartElo || len(a.Welcome.Ladder) != len(ranked.Ladder) {
		t.Fatalf("welcome elo %d, %d ranks", a.Welcome.Elo, len(a.Welcome.Ladder))
	}

	a.Send(t, protocol.ClientDuelChallenge, protocol.DuelChallengeReq{Target: b.Welcome.EntityID, Ranked: true})
	ch := testkit.Expect[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	if !ch.Ranked || ch.Stakes == nil || ch.Stakes.WinMin <= 0 {
		t.Fatalf("challenge %+v", ch)
	}
	accept(t, a, b, ch)
	a.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "crane"})
	end := testkit.Expect[protocol.DuelEnd](t, a, protocol.ServerDuelEnd)
	if !end.Ranked || end.EloBefore != ranked.StartElo || end.EloAfter <= end.EloBefore {
		t.Fatalf("a's end %+v", end)
	}
	lost := testkit.Expect[protocol.DuelEnd](t, b, protocol.ServerDuelEnd)
	if lost.EloAfter >= ranked.StartElo {
		t.Fatalf("b's end %+v", lost)
	}

	// Reconnecting right away loads the saved rating
	a.Close()
	a2 := ts.Dial(t, a.Account)
	if a2.Welcome.Elo != end.EloAfter {
		t.Fatalf("after reconnecting elo is %d, want %d", a2.Welcome.Elo, end.EloAfter)
	}
	// and others see it on the new entity
	view := b.WatchWorld(testkit.Timeout, func(v testkit.View) bool { _, ok := v.Spawns[a2.Welcome.EntityID]; return ok })
	if s := view.Spawns[a2.Welcome.EntityID]; s.Elo != end.EloAfter {
		t.Fatalf("b sees a at %d", s.Elo)
	}

	b.Send(t, protocol.ClientProfile, protocol.ProfileReq{Target: a2.Welcome.EntityID})
	prof := testkit.Expect[protocol.Profile](t, b, protocol.ServerProfile)
	if prof.Elo != end.EloAfter || prof.Games != 1 || prof.Wins != 1 || prof.Stakes == nil {
		t.Fatalf("profile %+v", prof)
	}
	if len(prof.Recent) != 1 || prof.Recent[0].Outcome != protocol.DuelWin || prof.Recent[0].Opponent != b.Account.Username || prof.Recent[0].Change != end.EloAfter-end.EloBefore {
		t.Fatalf("recent %+v", prof.Recent)
	}
	b.Send(t, protocol.ClientProfile, protocol.ProfileReq{Target: b.Welcome.EntityID})
	if own := testkit.Expect[protocol.Profile](t, b, protocol.ServerProfile); own.Losses != 1 || own.Stakes != nil {
		t.Fatalf("own profile %+v", own)
	}
}

func TestProfileIsRateLimited(t *testing.T) {
	t.Parallel()
	_, cs := duelists(t, "a", "b")
	a, b := cs[0], cs[1]
	for i := 0; i < 20; i++ {
		a.Send(t, protocol.ClientProfile, protocol.ProfileReq{Target: b.Welcome.EntityID})
	}
	if n := a.Count(protocol.ServerProfile, 500*time.Millisecond); n < 1 || n > 5+1 {
		t.Fatalf("%d profiles for a burst of 20", n)
	}
}
