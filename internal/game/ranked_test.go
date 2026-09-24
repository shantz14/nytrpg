package game

import (
	"runtime"
	"testing"
	"time"

	"nytrpg/internal/protocol"
	"nytrpg/internal/ranked"
)

// Established players (past the provisional games) at the given elo
func (tw *testWorld) rate(p *player, elo int) {
	p.rating = ranked.Rating{Elo: elo, Peak: elo, Games: 50}
	p.ent.Elo = elo
}

type fakeHistory struct{ asked []int }

func (h *fakeHistory) Recent(charID int) []protocol.RankedMatchInfo {
	h.asked = append(h.asked, charID)
	return []protocol.RankedMatchInfo{{Opponent: "someone", Change: 12}}
}

// Profile waits on the world, which tests run by hand: flush until it answers
func (tw *testWorld) profile(c Client, target protocol.EntityID) (protocol.Profile, bool) {
	type result struct {
		prof protocol.Profile
		ok   bool
	}
	done := make(chan result)
	go func() {
		prof, ok := tw.Profile(c, target)
		done <- result{prof, ok}
	}()
	for {
		select {
		case r := <-done:
			return r.prof, r.ok
		default:
			tw.flush()
			runtime.Gosched()
		}
	}
}

func TestRankedDuelSettles(t *testing.T) {
	tw := newDuelWorld()
	var matches []ranked.Match
	tw.OnRanked = func(m ranked.Match) { matches = append(matches, m) }
	a, pa, b, pb := tw.pair()
	watcher, _ := tw.join(3, protocol.Vec{X: 550, Y: 550})
	tw.rate(pa, 1200)
	tw.rate(pb, 1000)
	tw.tickNow()
	a.msgs, b.msgs, watcher.msgs = nil, nil, nil

	tw.Challenge(a, pb.ent.ID, true)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	want := ranked.Stakes(pb.rating, pa.rating)
	if !ch.Ranked || ch.Elo != 1200 || ch.Stakes == nil || *ch.Stakes != want {
		t.Fatalf("ranked challenge %+v, want stakes %+v", ch, want)
	}
	tw.RespondDuel(b, ch.ID, true)
	tw.flush()
	if s := one[protocol.DuelStart](t, a, protocol.ServerDuelStart); !s.Ranked {
		t.Fatal("start should say ranked")
	}

	tw.advance(30 * time.Second)
	tw.DuelGuess(b, "crisp") // two greens: 0.4 of the way there
	tw.DuelGuess(a, "crane")
	tw.flush()
	endA := one[protocol.DuelEnd](t, a, protocol.ServerDuelEnd)
	endB := one[protocol.DuelEnd](t, b, protocol.ServerDuelEnd)
	if !endA.Ranked || endA.EloBefore != 1200 || endA.EloAfter <= 1200 || endB.EloBefore != 1000 || endB.EloAfter >= 1000 {
		t.Fatalf("ends %+v %+v", endA, endB)
	}
	if endA.EloAfter-1200 != 1000-endB.EloAfter {
		t.Fatal("established players trade the same points")
	}
	if endA.Expected < 0.7 || endA.Expected+endB.Expected != 1 || endA.Margin <= 1 || endA.Margin != endB.Margin {
		t.Fatalf("breakdown a=%v/%v b=%v", endA.Expected, endA.Margin, endB.Expected)
	}
	m := ranked.Margin{WinnerGuesses: 1, WinnerSeconds: 30, LoserGuesses: 1, LoserBest: 0.4}
	if endA.Margin != m.Multiplier() {
		t.Fatalf("margin %v, want %v from the duel", endA.Margin, m.Multiplier())
	}
	if pa.rating.Elo != endA.EloAfter || pa.rating.Wins != 1 || pb.rating.Losses != 1 || pa.ent.Elo != endA.EloAfter {
		t.Fatalf("ratings in memory %+v %+v", pa.rating, pb.rating)
	}

	if len(matches) != 1 {
		t.Fatalf("%d matches saved", len(matches))
	}
	mt := matches[0]
	if mt.A != pa.charID || mt.B != pb.charID || mt.Outcome != ranked.AWins || mt.AGuesses != 1 || mt.BGuesses != 1 || mt.Result.A.Elo != endA.EloAfter {
		t.Fatalf("match %+v", mt)
	}

	// Everyone who can see them gets the new elo next tick, once
	tw.tickNow()
	elos := map[protocol.EntityID]int{}
	for _, u := range watcher.updates(t) {
		for _, e := range u.Elo {
			elos[e.ID] = e.Elo
		}
	}
	if elos[pa.ent.ID] != endA.EloAfter || elos[pb.ent.ID] != endB.EloAfter {
		t.Fatalf("watcher saw %v", elos)
	}
	tw.tickNow()
	for _, u := range watcher.updates(t) {
		if len(u.Elo) != 0 {
			t.Fatal("elo sent again without changing")
		}
	}
	// And someone arriving later gets it in the spawn
	late, _ := tw.join(4, protocol.Vec{X: 520, Y: 520})
	tw.tickNow()
	seen := false
	for _, u := range late.updates(t) {
		for _, s := range u.Spawn {
			if s.ID == pa.ent.ID {
				seen = s.Elo == endA.EloAfter
			}
		}
	}
	if !seen {
		t.Fatal("a late arrival should spawn the winner with the new elo")
	}
}

func TestCasualDuelLeavesEloAlone(t *testing.T) {
	tw := newDuelWorld()
	saved := 0
	tw.OnRanked = func(ranked.Match) { saved++ }
	a, pa, b, pb := tw.pair()
	tw.startDuel(t, a, b, pb)
	tw.DuelGuess(a, "crane")
	tw.flush()
	end := one[protocol.DuelEnd](t, a, protocol.ServerDuelEnd)
	if end.Ranked || end.EloAfter != 0 || saved != 0 || pa.rating != ranked.NewRating() || pa.ent.eloChanged {
		t.Fatalf("casual duel touched ratings: %+v %+v", end, pa.rating)
	}
}

func TestRankedForfeitAndDisconnectCostTheMost(t *testing.T) {
	for _, leave := range []bool{false, true} {
		tw := newDuelWorld()
		a, pa, b, pb := tw.pair()
		tw.rate(pa, 1000)
		tw.rate(pb, 1000)
		tw.Challenge(a, pb.ent.ID, true)
		tw.flush()
		ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
		tw.RespondDuel(b, ch.ID, true)
		tw.flush()
		b.msgs = nil

		if leave {
			tw.Leave(a)
		} else {
			tw.ForfeitDuel(a)
		}
		tw.flush()
		end := one[protocol.DuelEnd](t, b, protocol.ServerDuelEnd)
		if end.Margin != ranked.MaxMargin || end.EloAfter-end.EloBefore != ch.Stakes.WinMax {
			t.Fatalf("leave=%v: %+v, stakes %+v", leave, end, ch.Stakes)
		}
	}
}

func TestRankedChallengeReplacesCasual(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	tw.Challenge(a, pb.ent.ID, false)
	tw.Challenge(a, pb.ent.ID, true)
	tw.flush()
	chs := got[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	if len(chs) != 2 || chs[0].Ranked || !chs[1].Ranked {
		t.Fatalf("challenges %+v", chs)
	}
	// The casual one was taken down
	var cancelled bool
	for _, u := range got[protocol.DuelChallengeUpdate](t, b, protocol.ServerDuelChallengeUpdate) {
		cancelled = cancelled || (u.ID == chs[0].ID && u.Status == protocol.DuelCancelled)
	}
	if !cancelled {
		t.Fatal("switching to ranked should cancel the casual challenge")
	}
}

func TestProfile(t *testing.T) {
	tw := newDuelWorld()
	hist := &fakeHistory{}
	tw.History = hist
	a, pa, _, pb := tw.pair()
	far, pfar := tw.join(3, protocol.Vec{X: 4500, Y: 4500})
	tw.rate(pa, 1100)
	tw.rate(pb, 1450)
	pb.rating.Wins, pb.rating.Losses, pb.rating.Peak = 30, 20, 1500
	tw.tickNow()

	self, ok := tw.profile(a, pa.ent.ID)
	if !ok || self.Elo != 1100 || self.Char != "char1" || self.Stakes != nil {
		t.Fatalf("own profile %+v", self)
	}
	other, ok := tw.profile(a, pb.ent.ID)
	want := ranked.Stakes(pa.rating, pb.rating)
	if !ok || other.Elo != 1450 || other.Peak != 1500 || other.Wins != 30 || other.Games != 50 || other.Stakes == nil || *other.Stakes != want {
		t.Fatalf("their profile %+v", other)
	}
	if len(other.Recent) != 1 || hist.asked[len(hist.asked)-1] != pb.charID {
		t.Fatalf("recent duels come from the history, for their character: %+v %v", other.Recent, hist.asked)
	}
	if _, ok := tw.profile(a, pfar.ent.ID); ok {
		t.Fatal("profile of someone out of view")
	}
	if _, ok := tw.profile(far, 9999); ok {
		t.Fatal("profile of nobody")
	}
}
