package server_test

import (
	"reflect"
	"testing"
	"time"

	"nytrpg/internal/protocol"
	"nytrpg/internal/testkit"
)

// Connects players who can all see each other, on a server whose duels use CRANE
func duelists(t *testing.T, names ...string) (*testkit.Server, []*testkit.Client) {
	t.Helper()
	ts := testkit.NewServer(t)
	ts.SetDuelWord("CRANE")
	var cs []*testkit.Client
	for _, n := range names {
		cs = append(cs, ts.Connect(t, testkit.UniqueName(n)))
	}
	// Challenges need the target in view, which takes a world tick
	for _, c := range cs {
		c.WatchWorld(testkit.Timeout, func(v testkit.View) bool {
			for _, o := range cs {
				if _, ok := v.Pos[o.Welcome.EntityID]; !ok && o != c {
					return false
				}
			}
			return true
		})
	}
	return ts, cs
}

func challenge(t *testing.T, from, to *testkit.Client) protocol.DuelChallenge {
	t.Helper()
	from.Send(t, protocol.ClientDuelChallenge, protocol.DuelChallengeReq{Target: to.Welcome.EntityID})
	ch := testkit.Expect[protocol.DuelChallenge](t, to, protocol.ServerDuelChallenge)
	if u := testkit.Expect[protocol.DuelChallengeUpdate](t, from, protocol.ServerDuelChallengeUpdate); u.Status != protocol.DuelSent || u.ID != ch.ID {
		t.Fatalf("challenger got %+v", u)
	}
	return ch
}

func accept(t *testing.T, a, b *testkit.Client, ch protocol.DuelChallenge) {
	t.Helper()
	b.Send(t, protocol.ClientDuelRespond, protocol.DuelRespondReq{ID: ch.ID, Accept: true})
	testkit.Expect[protocol.DuelStart](t, a, protocol.ServerDuelStart)
	testkit.Expect[protocol.DuelStart](t, b, protocol.ServerDuelStart)
}

func TestDuelFullFlow(t *testing.T) {
	t.Parallel()
	ts, cs := duelists(t, "a", "b")
	a, b := cs[0], cs[1]

	ch := challenge(t, a, b)
	if ch.From != a.Welcome.EntityID || ch.Name != a.Account.Username {
		t.Fatalf("challenge %+v", ch)
	}
	b.Send(t, protocol.ClientDuelRespond, protocol.DuelRespondReq{ID: ch.ID, Accept: true})
	start := testkit.Expect[protocol.DuelStart](t, a, protocol.ServerDuelStart)
	if start.Name != b.Account.Username || start.WordLength != 5 || start.MaxGuesses != 5 {
		t.Fatalf("a's start %+v", start)
	}
	testkit.Expect[protocol.DuelStart](t, b, protocol.ServerDuelStart)

	// b watches a type, then guess
	a.Send(t, protocol.ClientDuelTyping, protocol.DuelTyping{Count: 3})
	if ty := testkit.Expect[protocol.DuelTyping](t, b, protocol.ServerDuelTyping); ty.Count != 3 {
		t.Fatalf("typing %+v", ty)
	}
	a.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "slate"})
	if res := testkit.Expect[protocol.WordleRes](t, a, protocol.ServerDuelGuess); !res.Valid || res.Status != protocol.WordleInGame {
		t.Fatalf("a's guess %+v", res)
	}
	og := testkit.Expect[protocol.DuelOpponentGuess](t, b, protocol.ServerDuelOpponentGuess)
	want := []protocol.WordleColor{protocol.Grey, protocol.Grey, protocol.Green, protocol.Grey, protocol.Green}
	if !reflect.DeepEqual(og.Colors, want) {
		t.Fatalf("b saw %v, want %v", og.Colors, want)
	}

	b.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "crane"})
	if end := testkit.Expect[protocol.DuelEnd](t, b, protocol.ServerDuelEnd); end.Outcome != protocol.DuelWin || end.Solution != "CRANE" {
		t.Fatalf("b's end %+v", end)
	}
	if end := testkit.Expect[protocol.DuelEnd](t, a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelLose || end.Reason != protocol.DuelSolved {
		t.Fatalf("a's end %+v", end)
	}

	// Duels don't touch the daily wordle or its leaderboard
	a.Send(t, protocol.ClientWordleStart, nil)
	if r := testkit.Expect[protocol.WordleResume](t, a, protocol.ServerWordleResume); r.Played || len(r.Guesses) != 0 {
		t.Fatalf("daily wordle after a duel: %+v", r)
	}
	var lb struct {
		Rows []any `json:"rows"`
	}
	ts.GetJSON(t, "/leaderboard", &lb)
	if len(lb.Rows) != 0 {
		t.Fatalf("duel ended up on the leaderboard: %+v", lb.Rows)
	}
}

func TestDuelResponsesMustBeYours(t *testing.T) {
	t.Parallel()
	_, cs := duelists(t, "a", "b", "c")
	a, b, c := cs[0], cs[1], cs[2]

	ch := challenge(t, a, b)
	// A bystander can't accept for b, nor can a accept their own, nor a made up id
	c.Send(t, protocol.ClientDuelRespond, protocol.DuelRespondReq{ID: ch.ID, Accept: true})
	a.Send(t, protocol.ClientDuelRespond, protocol.DuelRespondReq{ID: ch.ID, Accept: true})
	b.Send(t, protocol.ClientDuelRespond, protocol.DuelRespondReq{ID: ch.ID + 100, Accept: true})
	a.ExpectNone(t, protocol.ServerDuelStart, 300*time.Millisecond)

	accept(t, a, b, ch)
	c.ExpectNone(t, protocol.ServerDuelStart, 100*time.Millisecond)
}

func TestDuelChallengesAreRateLimited(t *testing.T) {
	t.Parallel()
	_, cs := duelists(t, "a", "b", "c")
	a, b, c := cs[0], cs[1], cs[2]
	// Switching targets makes every challenge new, so only the limit stops them
	for i := 0; i < 10; i++ {
		to := b
		if i%2 == 1 {
			to = c
		}
		a.Send(t, protocol.ClientDuelChallenge, protocol.DuelChallengeReq{Target: to.Welcome.EntityID})
	}
	if n := a.Count(protocol.ServerDuelChallengeUpdate, 500*time.Millisecond); n > 3+1 {
		t.Fatalf("%d challenges went through a burst of 3", n)
	}
}

func TestDuelDisconnectLoses(t *testing.T) {
	t.Parallel()
	_, cs := duelists(t, "a", "b")
	a, b := cs[0], cs[1]
	accept(t, a, b, challenge(t, a, b))

	b.Close()
	if end := testkit.Expect[protocol.DuelEnd](t, a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelWin || end.Reason != protocol.DuelDisconnect {
		t.Fatalf("a's end %+v", end)
	}
}

func TestDuelTypingFloodIsDroppedNotKicked(t *testing.T) {
	t.Parallel()
	_, cs := duelists(t, "a", "b")
	a, b := cs[0], cs[1]
	accept(t, a, b, challenge(t, a, b))

	for i := 0; i < 40; i++ {
		a.Send(t, protocol.ClientDuelTyping, protocol.DuelTyping{Count: i % 6})
	}
	if n := b.Count(protocol.ServerDuelTyping, 300*time.Millisecond); n < 1 || n > 20 {
		t.Fatalf("b got %d typing updates from a burst of 40", n)
	}
	// a is still connected and dueling
	a.Send(t, protocol.ClientDuelGuess, protocol.WordleReq{Guess: "crane"})
	if end := testkit.Expect[protocol.DuelEnd](t, a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelWin {
		t.Fatalf("a's end %+v", end)
	}
}
