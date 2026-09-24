package game

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"nytrpg/internal/protocol"
)

// Always CRANE. Any five letters are a word except XXXXX. Letters in the right
// place are green, the rest grey.
type fakePuzzle struct{}

func (fakePuzzle) NewWord(*rand.Rand) string { return "CRANE" }
func (fakePuzzle) MaxGuesses() int           { return 3 }
func (fakePuzzle) Score(guess, word string) (bool, []protocol.WordleColor) {
	guess = strings.ToUpper(guess)
	if len(guess) != len(word) || guess == "XXXXX" {
		return false, nil
	}
	colors := make([]protocol.WordleColor, len(word))
	for i := range word {
		if guess[i] == word[i] {
			colors[i] = protocol.Green
		}
	}
	return true, colors
}

func newDuelWorld() *testWorld {
	tw := newTestWorld()
	tw.Duels = fakePuzzle{}
	return tw
}

// Every message of type typ c got, decoded, and forgets them
func got[T any](t *testing.T, c *fakeClient, typ protocol.ServerMsg) []T {
	t.Helper()
	var out []T
	for _, raw := range c.take(typ) {
		var v T
		if err := msgpack.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func one[T any](t *testing.T, c *fakeClient, typ protocol.ServerMsg) T {
	t.Helper()
	all := got[T](t, c, typ)
	if len(all) != 1 {
		t.Fatalf("got %d messages of type %d, want 1", len(all), typ)
	}
	return all[0]
}

func none(t *testing.T, c *fakeClient, typ protocol.ServerMsg) {
	t.Helper()
	if n := len(c.take(typ)); n != 0 {
		t.Fatalf("got %d messages of type %d, want none", n, typ)
	}
}

// Two players standing together who have seen each other
func (tw *testWorld) pair() (a *fakeClient, pa *player, b *fakeClient, pb *player) {
	a, pa = tw.join(1, protocol.Vec{X: 500, Y: 500})
	b, pb = tw.join(2, protocol.Vec{X: 600, Y: 500})
	tw.tickNow()
	a.msgs, b.msgs = nil, nil
	return
}

// a challenges b, b accepts. Returns with no messages left.
func (tw *testWorld) startDuel(t *testing.T, a, b *fakeClient, pb *player) {
	t.Helper()
	tw.Challenge(a, pb.ent.ID)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	tw.RespondDuel(b, ch.ID, true)
	tw.flush()
	one[protocol.DuelStart](t, a, protocol.ServerDuelStart)
	one[protocol.DuelStart](t, b, protocol.ServerDuelStart)
	a.msgs, b.msgs = nil, nil
}

func TestChallengeAndAccept(t *testing.T) {
	tw := newDuelWorld()
	a, pa, b, pb := tw.pair()

	tw.Challenge(a, pb.ent.ID)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	if ch.From != pa.ent.ID || ch.Char != "char1" || ch.Class != "wizard" || ch.ExpiresMs != 30000 {
		t.Fatalf("challenge %+v", ch)
	}
	sent := one[protocol.DuelChallengeUpdate](t, a, protocol.ServerDuelChallengeUpdate)
	if sent.Status != protocol.DuelSent || sent.ID != ch.ID || sent.Name != "char2" {
		t.Fatalf("challenger got %+v", sent)
	}

	tw.RespondDuel(b, ch.ID, true)
	tw.flush()
	for _, c := range []struct {
		client *fakeClient
		vs     string
	}{{a, "char2"}, {b, "char1"}} {
		start := one[protocol.DuelStart](t, c.client, protocol.ServerDuelStart)
		if start.Char != c.vs || start.WordLength != 5 || start.MaxGuesses != 3 {
			t.Fatalf("start %+v", start)
		}
	}
	if pa.duel == nil || pa.duel != pb.duel {
		t.Fatal("both players should share the duel")
	}
}

func TestChallengeRules(t *testing.T) {
	tw := newDuelWorld()
	a, pa, b, pb := tw.pair()
	far, pfar := tw.join(3, protocol.Vec{X: 4500, Y: 4500})
	tw.tickNow()
	a.msgs, b.msgs, far.msgs = nil, nil, nil

	status := func() protocol.DuelChallengeStatus {
		t.Helper()
		tw.flush()
		return one[protocol.DuelChallengeUpdate](t, a, protocol.ServerDuelChallengeUpdate).Status
	}

	tw.Challenge(a, pa.ent.ID)
	if s := status(); s != protocol.DuelUnavailable {
		t.Fatalf("challenging yourself: %d", s)
	}
	tw.Challenge(a, pfar.ent.ID)
	if s := status(); s != protocol.DuelUnavailable {
		t.Fatalf("challenging someone out of view: %d", s)
	}
	none(t, far, protocol.ServerDuelChallenge)
	tw.Challenge(a, 9999)
	if s := status(); s != protocol.DuelUnavailable {
		t.Fatalf("challenging nobody: %d", s)
	}

	// Asking twice while waiting does nothing
	tw.Challenge(a, pb.ent.ID)
	tw.Challenge(a, pb.ent.ID)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)

	// Anyone in a duel is busy
	c, pc := tw.join(4, protocol.Vec{X: 550, Y: 550})
	tw.tickNow()
	tw.RespondDuel(b, ch.ID, true)
	tw.flush()
	a.msgs, b.msgs = nil, nil
	tw.Challenge(c, pa.ent.ID)
	tw.flush()
	if s := one[protocol.DuelChallengeUpdate](t, c, protocol.ServerDuelChallengeUpdate).Status; s != protocol.DuelBusy {
		t.Fatalf("challenging someone dueling: %d", s)
	}
	tw.Challenge(a, pc.ent.ID)
	if s := status(); s != protocol.DuelBusy {
		t.Fatalf("challenging while dueling: %d", s)
	}
	none(t, c, protocol.ServerDuelChallenge)
}

func TestChallengeExpiresAfter30s(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	tw.Challenge(a, pb.ent.ID)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	a.msgs = nil

	tw.advance(ChallengeTimeout - time.Millisecond)
	tw.tickNow()
	none(t, a, protocol.ServerDuelChallengeUpdate)

	tw.advance(time.Millisecond)
	tw.tickNow()
	for _, c := range []*fakeClient{a, b} {
		if u := one[protocol.DuelChallengeUpdate](t, c, protocol.ServerDuelChallengeUpdate); u.Status != protocol.DuelExpired || u.ID != ch.ID {
			t.Fatalf("expiry %+v", u)
		}
	}
	// Too late to accept
	tw.RespondDuel(b, ch.ID, true)
	tw.flush()
	none(t, a, protocol.ServerDuelStart)
	none(t, b, protocol.ServerDuelStart)
}

func TestOnlyTheTargetCanRespond(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	c, _ := tw.join(3, protocol.Vec{X: 550, Y: 550})
	tw.Challenge(a, pb.ent.ID)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)

	// Neither the challenger nor a bystander can accept for them
	tw.RespondDuel(a, ch.ID, true)
	tw.RespondDuel(c, ch.ID, true)
	tw.RespondDuel(b, ch.ID+1, true)
	tw.flush()
	none(t, a, protocol.ServerDuelStart)
	none(t, b, protocol.ServerDuelStart)
	none(t, c, protocol.ServerDuelStart)

	// The challenge is still there for the target
	tw.RespondDuel(b, ch.ID, false)
	tw.flush()
	var declined bool
	for _, u := range got[protocol.DuelChallengeUpdate](t, a, protocol.ServerDuelChallengeUpdate) {
		declined = declined || (u.ID == ch.ID && u.Status == protocol.DuelDeclined)
	}
	if !declined {
		t.Fatal("challenger should hear the decline")
	}
	none(t, b, protocol.ServerDuelStart)
}

func TestAcceptingCancelsOtherChallenges(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	c, pc := tw.join(3, protocol.Vec{X: 550, Y: 550})
	tw.tickNow()
	a.msgs, b.msgs, c.msgs = nil, nil, nil

	// c challenges b too, and b had challenged c
	tw.Challenge(c, pb.ent.ID)
	tw.Challenge(b, pc.ent.ID)
	tw.flush()
	fromC := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	b.msgs, c.msgs = nil, nil

	tw.startDuel(t, a, b, pb)
	// startDuel threw the updates away, so check c's side of it directly
	tw.RespondDuel(b, fromC.ID, true)
	tw.flush()
	none(t, c, protocol.ServerDuelStart)
	if len(tw.challenges) != 0 {
		t.Fatalf("%d challenges left", len(tw.challenges))
	}
}

func TestAcceptingTellsOthersTheirChallengeIsOff(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	c, _ := tw.join(3, protocol.Vec{X: 550, Y: 550})
	tw.tickNow()
	c.msgs = nil

	tw.Challenge(c, pb.ent.ID)
	tw.Challenge(a, pb.ent.ID)
	tw.flush()
	var fromA protocol.DuelChallenge
	for _, ch := range got[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge) {
		if ch.Char == "char1" {
			fromA = ch
		}
	}
	c.msgs = nil
	tw.RespondDuel(b, fromA.ID, true)
	tw.flush()
	if u := one[protocol.DuelChallengeUpdate](t, c, protocol.ServerDuelChallengeUpdate); u.Status != protocol.DuelCancelled {
		t.Fatalf("c got %+v", u)
	}
}

func TestOpponentSeesColorsNotLetters(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	tw.startDuel(t, a, b, pb)

	tw.DuelGuess(a, "crisp")
	tw.flush()
	res := one[protocol.WordleRes](t, a, protocol.ServerDuelGuess)
	if !res.Valid || res.Status != protocol.WordleInGame || res.Solution != "" {
		t.Fatalf("guesser got %+v", res)
	}
	raw := b.take(protocol.ServerDuelOpponentGuess)
	if len(raw) != 1 {
		t.Fatalf("opponent got %d guesses", len(raw))
	}
	if bytes.Contains(bytes.ToUpper(raw[0]), []byte("CRISP")) || bytes.Contains(bytes.ToUpper(raw[0]), []byte("RIS")) {
		t.Fatal("the opponent's message has the letters in it")
	}
	var og protocol.DuelOpponentGuess
	if err := msgpack.Unmarshal(raw[0], &og); err != nil {
		t.Fatal(err)
	}
	want := []protocol.WordleColor{protocol.Green, protocol.Green, protocol.Grey, protocol.Grey, protocol.Grey}
	for i := range want {
		if og.Colors[i] != want[i] {
			t.Fatalf("colors %v, want %v", og.Colors, want)
		}
	}

	// Words that aren't words cost nothing and aren't forwarded
	tw.DuelGuess(a, "xxxxx")
	tw.flush()
	if res := one[protocol.WordleRes](t, a, protocol.ServerDuelGuess); res.Valid {
		t.Fatal("invalid guess was accepted")
	}
	none(t, b, protocol.ServerDuelOpponentGuess)
}

func TestFirstToSolveWins(t *testing.T) {
	tw := newDuelWorld()
	a, pa, b, pb := tw.pair()
	tw.startDuel(t, a, b, pb)

	tw.advance(42 * time.Second)
	tw.DuelGuess(b, "crane")
	tw.flush()
	if res := one[protocol.WordleRes](t, b, protocol.ServerDuelGuess); res.Status != protocol.WordleWin {
		t.Fatalf("solver got %+v", res)
	}
	endB := one[protocol.DuelEnd](t, b, protocol.ServerDuelEnd)
	endA := one[protocol.DuelEnd](t, a, protocol.ServerDuelEnd)
	if endB.Outcome != protocol.DuelWin || endA.Outcome != protocol.DuelLose {
		t.Fatalf("outcomes b=%d a=%d", endB.Outcome, endA.Outcome)
	}
	if endA.Reason != protocol.DuelSolved || endA.Solution != "CRANE" || endA.Seconds != 42 {
		t.Fatalf("end %+v", endA)
	}
	if pa.duel != nil || pb.duel != nil {
		t.Fatal("duel should be over for both")
	}

	// Guesses and typing after the end go nowhere
	tw.DuelGuess(a, "crane")
	tw.DuelTyping(a, 3)
	tw.flush()
	none(t, a, protocol.ServerDuelGuess)
	none(t, b, protocol.ServerDuelTyping)
}

func TestOutOfGuessesWaitsForOpponent(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	tw.startDuel(t, a, b, pb)

	for i := 0; i < 3; i++ {
		tw.DuelGuess(a, "blimp")
	}
	tw.flush()
	all := got[protocol.WordleRes](t, a, protocol.ServerDuelGuess)
	if all[2].Status != protocol.WordleLose {
		t.Fatalf("last guess %+v", all[2])
	}
	none(t, a, protocol.ServerDuelEnd)
	// Out means no more guesses
	tw.DuelGuess(a, "crane")
	tw.flush()
	none(t, a, protocol.ServerDuelGuess)

	// b can still win
	tw.DuelGuess(b, "crane")
	tw.flush()
	if end := one[protocol.DuelEnd](t, a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelLose {
		t.Fatalf("a got %+v", end)
	}
	if end := one[protocol.DuelEnd](t, b, protocol.ServerDuelEnd); end.Outcome != protocol.DuelWin {
		t.Fatalf("b got %+v", end)
	}
}

func TestBothOutIsADraw(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	tw.startDuel(t, a, b, pb)
	for i := 0; i < 3; i++ {
		tw.DuelGuess(a, "blimp")
		tw.DuelGuess(b, "fjord")
	}
	tw.flush()
	for _, c := range []*fakeClient{a, b} {
		end := one[protocol.DuelEnd](t, c, protocol.ServerDuelEnd)
		if end.Outcome != protocol.DuelDraw || end.Reason != protocol.DuelOutOfGuesses {
			t.Fatalf("end %+v", end)
		}
	}
}

func TestForfeitAndLeave(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()
	tw.startDuel(t, a, b, pb)
	tw.ForfeitDuel(a)
	tw.flush()
	if end := one[protocol.DuelEnd](t, b, protocol.ServerDuelEnd); end.Outcome != protocol.DuelWin || end.Reason != protocol.DuelForfeit {
		t.Fatalf("b got %+v", end)
	}
	if end := one[protocol.DuelEnd](t, a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelLose {
		t.Fatalf("a got %+v", end)
	}

	// Leaving mid duel loses it, and pending challenges from you are off
	c, _ := tw.join(3, protocol.Vec{X: 550, Y: 550})
	tw.tickNow()
	a.msgs, b.msgs, c.msgs = nil, nil, nil
	tw.startDuel(t, a, b, pb)
	tw.Challenge(c, pb.ent.ID) // b is busy, refused
	tw.Leave(b)
	tw.flush()
	if end := one[protocol.DuelEnd](t, a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelWin || end.Reason != protocol.DuelDisconnect {
		t.Fatalf("a got %+v", end)
	}

	d, _ := tw.join(4, protocol.Vec{X: 560, Y: 560})
	tw.tickNow()
	c.msgs, d.msgs = nil, nil
	tw.Challenge(c, tw.players[d].ent.ID)
	tw.flush()
	d.msgs = nil
	tw.Leave(c)
	tw.flush()
	if u := one[protocol.DuelChallengeUpdate](t, d, protocol.ServerDuelChallengeUpdate); u.Status != protocol.DuelCancelled {
		t.Fatalf("d got %+v", u)
	}
	if len(tw.challenges) != 0 {
		t.Fatalf("%d challenges left", len(tw.challenges))
	}
}

func TestTypingForwardsOnlyTheCount(t *testing.T) {
	tw := newDuelWorld()
	a, _, b, pb := tw.pair()

	// Not in a duel yet
	tw.DuelTyping(a, 2)
	tw.flush()
	none(t, b, protocol.ServerDuelTyping)

	tw.startDuel(t, a, b, pb)
	tw.DuelTyping(a, 3)
	tw.DuelTyping(a, 0)
	tw.DuelTyping(a, -1)
	tw.DuelTyping(a, 6)
	tw.flush()
	counts := got[protocol.DuelTyping](t, b, protocol.ServerDuelTyping)
	if len(counts) != 2 || counts[0].Count != 3 || counts[1].Count != 0 {
		t.Fatalf("opponent got %+v", counts)
	}
	none(t, a, protocol.ServerDuelTyping)

	// Once you're out, you have nothing to type
	for i := 0; i < 3; i++ {
		tw.DuelGuess(a, "blimp")
	}
	tw.DuelTyping(a, 1)
	tw.flush()
	none(t, b, protocol.ServerDuelTyping)
}
