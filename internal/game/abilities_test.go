package game

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"

	"nytrpg/internal/classes"
	"nytrpg/internal/protocol"
)

// Scores like Wordle (greens, then yellows), accepts any letters but XXXXX.
// Hands out words in order, the first is the duel's.
type scoringPuzzle struct {
	words []string
	next  *int
}

func (p scoringPuzzle) NewWord(*rand.Rand) string {
	w := p.words[*p.next%len(p.words)]
	*p.next++
	return w
}
func (scoringPuzzle) MaxGuesses() int                { return 3 }
func (scoringPuzzle) IllusionWord(*rand.Rand) string { return "CAT" }
func (p scoringPuzzle) Score(guess, word string) (bool, []protocol.WordleColor) {
	if strings.ToUpper(guess) == "XXXXX" {
		return false, nil
	}
	return p.ScoreAny(guess, word)
}
func (scoringPuzzle) ScoreAny(guess, word string) (bool, []protocol.WordleColor) {
	guess = strings.ToUpper(guess)
	if len(guess) != len(word) {
		return false, nil
	}
	colors := make([]protocol.WordleColor, len(word))
	left := map[byte]int{}
	for i := range word {
		if guess[i] == word[i] {
			colors[i] = protocol.Green
		} else {
			left[word[i]]++
		}
	}
	for i := range word {
		if colors[i] != protocol.Green && left[guess[i]] > 0 {
			colors[i] = protocol.Yellow
			left[guess[i]]--
		}
	}
	return true, colors
}

type duelTest struct {
	*testWorld
	t      *testing.T
	a, b   *fakeClient
	pa, pb *player
}

// A duel on CRANE between a player of class ca and one of class cb, with no
// messages left
func newAbilityDuel(t *testing.T, ca, cb classes.ID, words ...string) *duelTest {
	t.Helper()
	tw := newTestWorld()
	tw.Duels = scoringPuzzle{words: append([]string{"CRANE"}, words...), next: new(int)}
	a, pa, b, pb := tw.pair()
	pa.ent.Class, pb.ent.Class = string(ca), string(cb)
	tw.startDuel(t, a, b, pb)
	return &duelTest{testWorld: tw, t: t, a: a, b: b, pa: pa, pb: pb}
}

func (dt *duelTest) side(p *player) *duelSide {
	me, _ := p.duel.sidesOf(p)
	return me
}

func (dt *duelTest) cast(c *fakeClient, slot int, req protocol.DuelCastReq) {
	req.Slot = slot
	dt.CastAbility(c, req)
	dt.flush()
}

func (dt *duelTest) guess(c *fakeClient, word string) protocol.WordleRes {
	dt.t.Helper()
	dt.DuelGuess(c, word)
	dt.flush()
	return one[protocol.WordleRes](dt.t, c, protocol.ServerDuelGuess)
}

// The last DuelState c got, and forgets the rest
func (dt *duelTest) state(c *fakeClient) protocol.DuelState {
	dt.t.Helper()
	all := got[protocol.DuelState](dt.t, c, protocol.ServerDuelState)
	if len(all) == 0 {
		dt.t.Fatal("no duel state")
	}
	return all[len(all)-1]
}

// Moves the clock and runs a tick
func (dt *duelTest) wait(d time.Duration) {
	dt.advance(d)
	dt.tickNow()
}

// Slots in the classes package
const (
	slotSlash, slotShields, slotDetermination, slotPommel, slotCripple = 0, 1, 2, 3, 4
	slotScry, slotEye, slotMissile, slotIllusion, slotReshape          = 0, 1, 2, 3, 4
)

func TestDuelStartsWithNoEnergyAndGainsOneEvery30s(t *testing.T) {
	tw := newTestWorld()
	tw.Duels = fakePuzzle{}
	a, _, b, pb := tw.pair()
	tw.Challenge(a, pb.ent.ID, false)
	tw.flush()
	ch := one[protocol.DuelChallenge](t, b, protocol.ServerDuelChallenge)
	tw.RespondDuel(b, ch.ID, true)
	tw.flush()
	dt := &duelTest{testWorld: tw, t: t, a: a, b: b}
	if s := dt.state(a); s.You.Energy != 0 || s.Them.Energy != 0 || s.You.Rows != 3 {
		t.Fatalf("start %+v", s)
	}

	dt.wait(29 * time.Second)
	none(t, a, protocol.ServerDuelState)
	dt.wait(time.Second)
	if s := dt.state(b); s.You.Energy != 1 || s.Them.Energy != 1 {
		t.Fatalf("after 30s %+v", s)
	}
	dt.wait(30 * time.Second)
	if s := dt.state(a); s.You.Energy != 2 {
		t.Fatalf("after 60s %+v", s)
	}
}

func TestLettersPayEnergyOnlyTheFirstTime(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Rogue)
	me := dt.side(dt.pa)
	me.rows = 10
	for _, step := range []struct {
		guess string
		want  int
	}{
		{"CLAMP", 4},  // C and A green, +2 each
		{"CLAMP", 4},  // nothing new
		{"ACORN", 6},  // R and N yellow for the first time, A and C already found
		{"TRACE", 10}, // R and E green in new places, A's place and C already found
	} {
		dt.guess(dt.a, step.guess)
		if me.energy != step.want {
			t.Fatalf("after %s: energy %d, want %d", step.guess, me.energy, step.want)
		}
	}
	// The opponent is told
	if s := dt.state(dt.b); s.Them.Energy != 10 {
		t.Fatalf("opponent sees %+v", s.Them)
	}
}

func TestCastingNeedsEnergyAFilledSlotAndADuel(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Rogue)
	dt.cast(dt.a, slotPommel, protocol.DuelCastReq{})
	none(t, dt.a, protocol.ServerDuelCast)
	if s := dt.state(dt.a); s.You.Energy != 0 {
		t.Fatalf("the caster is set straight: %+v", s)
	}
	if !dt.side(dt.pb).stunnedUntil.IsZero() {
		t.Fatal("cast without energy")
	}

	dt.side(dt.pb).energy = 50
	for _, slot := range []int{-1, 5, 99} {
		// Nobody has slots outside 0-4
		dt.cast(dt.b, slot, protocol.DuelCastReq{})
		none(t, dt.b, protocol.ServerDuelCast)
	}
	dt.pb.ent.Class = "bard"
	for _, slot := range []int{0, 4} {
		// An unknown class has no abilities
		dt.cast(dt.b, slot, protocol.DuelCastReq{})
		none(t, dt.b, protocol.ServerDuelCast)
	}
	if dt.side(dt.pb).energy != 50 {
		t.Fatal("energy spent on nothing")
	}

	dt.ForfeitDuel(dt.b)
	dt.flush()
	dt.a.msgs = nil
	dt.cast(dt.a, slotShields, protocol.DuelCastReq{})
	if len(dt.a.msgs) != 0 {
		t.Fatal("cast after the duel ended")
	}
}

func TestSlashDestroysALetterTheOpponentGuessed(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Rogue)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	me.energy = 10

	// Nothing to hit before they guess
	dt.cast(dt.a, slotSlash, protocol.DuelCastReq{Row: 0, Col: 0})
	if me.energy != 10 {
		t.Fatal("slashed an empty row")
	}
	dt.guess(dt.b, "CLAMP")
	dt.a.msgs, dt.b.msgs = nil, nil

	dt.cast(dt.a, slotSlash, protocol.DuelCastReq{Row: 0, Col: 2})
	for _, c := range []*fakeClient{dt.a, dt.b} {
		ev := one[protocol.DuelCast](t, c, protocol.ServerDuelCast)
		if ev.Ability != classes.Slash || ev.ByYou != (c == dt.a) || ev.Row != 0 || ev.Col != 2 || ev.Blocked {
			t.Fatalf("slash event %+v", ev)
		}
	}
	if me.energy != 8 || !them.destroyed[tile{0, 2}] {
		t.Fatalf("energy %d destroyed %v", me.energy, them.destroyed)
	}
	if got := them.visibleColors()[0][2]; got != protocol.Hidden {
		t.Fatalf("destroyed tile shows %d", got)
	}

	for _, bad := range []protocol.DuelCastReq{{Row: 0, Col: 2}, {Row: 1, Col: 0}, {Row: 0, Col: 5}, {Row: -1, Col: 0}} {
		dt.cast(dt.a, slotSlash, bad)
		none(t, dt.b, protocol.ServerDuelCast)
	}
	if me.energy != 8 {
		t.Fatal("paid for a bad slash")
	}
}

func TestShieldBlocksTheNextHostileAbility(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Knight)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	me.energy, them.energy = 20, 20

	dt.cast(dt.b, slotShields, protocol.DuelCastReq{})
	if !them.shield || them.energy != 18 {
		t.Fatal("no shield")
	}
	if s := dt.state(dt.a); !s.Them.Shield {
		t.Fatal("the opponent should see the shield")
	}
	// One at a time
	dt.cast(dt.b, slotShields, protocol.DuelCastReq{})
	if them.energy != 18 {
		t.Fatal("second shield")
	}

	dt.a.msgs, dt.b.msgs = nil, nil
	dt.cast(dt.a, slotPommel, protocol.DuelCastReq{})
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); !ev.Blocked || ev.Ability != classes.PommelStrike {
		t.Fatalf("blocked event %+v", ev)
	}
	if !them.stunnedUntil.IsZero() || them.shield || me.energy != 17 {
		t.Fatalf("stun %v shield %v energy %d", them.stunnedUntil, them.shield, me.energy)
	}
	dt.cast(dt.a, slotPommel, protocol.DuelCastReq{})
	if them.stunnedUntil.IsZero() {
		t.Fatal("the shield only stops one")
	}
}

func TestStunBlocksGuessesAndTypingUntilItWearsOff(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Rogue)
	dt.side(dt.pa).energy = 3
	dt.cast(dt.a, slotPommel, protocol.DuelCastReq{})
	if s := dt.state(dt.b); s.You.StunnedMs != 10000 {
		t.Fatalf("stun %+v", s.You)
	}

	if res := dt.guess(dt.b, "CLAMP"); !res.Blocked || res.Valid {
		t.Fatalf("stunned guess %+v", res)
	}
	dt.DuelTyping(dt.b, 2)
	dt.flush()
	none(t, dt.a, protocol.ServerDuelTyping)

	dt.wait(9 * time.Second)
	if res := dt.guess(dt.b, "CLAMP"); !res.Blocked {
		t.Fatal("stun ended early")
	}
	dt.wait(time.Second)
	if s := dt.state(dt.b); s.You.StunnedMs != 0 {
		t.Fatalf("the end of the stun is sent %+v", s.You)
	}
	if res := dt.guess(dt.b, "CLAMP"); !res.Valid {
		t.Fatalf("after the stun %+v", res)
	}
}

func TestAggressiveStunsOnANewGreen(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Rogue)
	them := dt.side(dt.pb)

	dt.guess(dt.a, "PILOT") // nothing green
	if !them.stunnedUntil.IsZero() {
		t.Fatal("stunned without a green")
	}
	dt.guess(dt.a, "CLAMP")
	if !them.stunnedUntil.Equal(dt.clock.Add(AggressiveStun)) {
		t.Fatalf("stunned until %v", them.stunnedUntil)
	}
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); ev.Ability != classes.Aggressive || ev.Kind != protocol.CastTriggered || ev.ByYou {
		t.Fatalf("event %+v", ev)
	}

	dt.wait(AggressiveStun)
	them.stunnedUntil = time.Time{}
	dt.side(dt.pa).rows = 5
	dt.guess(dt.a, "CLAMP") // the same green again
	if !them.stunnedUntil.IsZero() {
		t.Fatal("stunned for a green found before")
	}

	// A shield stops it
	them.shield = true
	dt.guess(dt.a, "CRAMP")
	if !them.stunnedUntil.IsZero() || them.shield {
		t.Fatal("the shield should take the stun")
	}
}

func TestDeterminationGivesAnotherRowEvenWhenOut(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Rogue)
	me := dt.side(dt.pa)
	for range 2 {
		dt.guess(dt.a, "PILOT")
	}
	if res := dt.guess(dt.a, "PILOT"); res.Status != protocol.WordleLose {
		t.Fatalf("out %+v", res)
	}
	me.energy = 6
	dt.cast(dt.a, slotDetermination, protocol.DuelCastReq{})
	if s := dt.state(dt.b); s.Them.Rows != 4 || me.out() {
		t.Fatalf("rows %+v", s.Them)
	}
	if res := dt.guess(dt.a, "PILOT"); !res.Valid || res.Status != protocol.WordleLose {
		t.Fatalf("guess in the new row %+v", res)
	}

	// Out again, and the opponent runs out too: a draw
	for range 3 {
		dt.guess(dt.b, "PILOT")
	}
	if end := one[protocol.DuelEnd](t, dt.a, protocol.ServerDuelEnd); end.Outcome != protocol.DuelDraw {
		t.Fatalf("end %+v", end)
	}
}

func TestCrippleNeverTakesTheLastRow(t *testing.T) {
	dt := newAbilityDuel(t, classes.Knight, classes.Rogue)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	me.energy = 24
	dt.cast(dt.a, slotCripple, protocol.DuelCastReq{})
	dt.cast(dt.a, slotCripple, protocol.DuelCastReq{})
	if them.rows != 1 || me.energy != 8 {
		t.Fatalf("rows %d energy %d", them.rows, me.energy)
	}
	dt.cast(dt.a, slotCripple, protocol.DuelCastReq{})
	if them.rows != 1 || me.energy != 8 {
		t.Fatal("took their last row")
	}
}

func TestScryAnswersOnlyTheCaster(t *testing.T) {
	dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
	me := dt.side(dt.pa)
	me.energy = 10
	dt.cast(dt.a, slotScry, protocol.DuelCastReq{Letter: "r"})
	if s := one[protocol.DuelScry](t, dt.a, protocol.ServerDuelScry); s.Letter != "R" || !s.InWord {
		t.Fatalf("scry %+v", s)
	}
	dt.cast(dt.a, slotScry, protocol.DuelCastReq{Letter: "Z"})
	if s := one[protocol.DuelScry](t, dt.a, protocol.ServerDuelScry); s.InWord {
		t.Fatalf("scry %+v", s)
	}
	none(t, dt.b, protocol.ServerDuelScry)
	for _, bad := range []string{"", "AB", "1", "é"} {
		dt.cast(dt.a, slotScry, protocol.DuelCastReq{Letter: bad})
	}
	none(t, dt.a, protocol.ServerDuelScry)
	if me.energy != 6 {
		t.Fatalf("energy %d", me.energy)
	}
}

func TestSeeingEyeShowsNonGreenLettersAndMoves(t *testing.T) {
	dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
	me := dt.side(dt.pa)
	me.energy = 10

	// Nothing to see yet: the eye waits for a guess
	dt.cast(dt.a, slotEye, protocol.DuelCastReq{})
	if e := one[protocol.DuelEyes](t, dt.a, protocol.ServerDuelEyes); len(e.Tiles) != 0 {
		t.Fatalf("eyes %+v", e)
	}
	dt.guess(dt.b, "CLAMP") // C and A green, L M P not
	them := dt.side(dt.pb)
	check := func(e protocol.DuelEyes, n int) {
		t.Helper()
		if len(e.Tiles) != n {
			t.Fatalf("want %d eyes, got %+v", n, e)
		}
		seen := map[tile]bool{}
		for _, tl := range e.Tiles {
			at := tile{tl.Row, tl.Col}
			g := them.guesses[tl.Row]
			if seen[at] || g.colors[tl.Col] == protocol.Green || g.word[tl.Col:tl.Col+1] != tl.Letter {
				t.Fatalf("eye on %+v in %+v", tl, them.guesses)
			}
			seen[at] = true
		}
	}
	check(one[protocol.DuelEyes](t, dt.a, protocol.ServerDuelEyes), 1)

	dt.cast(dt.a, slotEye, protocol.DuelCastReq{})
	check(one[protocol.DuelEyes](t, dt.a, protocol.ServerDuelEyes), 2)
	if s := dt.state(dt.b); s.Them.Eyes != 2 {
		t.Fatalf("the opponent knows about the eyes %+v", s.Them)
	}

	for range 20 {
		dt.wait(EyeInterval)
		check(one[protocol.DuelEyes](t, dt.a, protocol.ServerDuelEyes), 2)
	}
}

func TestWiseMovesEyesAtOnce(t *testing.T) {
	dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
	me := dt.side(dt.pa)
	me.energy = 5
	dt.guess(dt.b, "PILOT")
	dt.cast(dt.a, slotEye, protocol.DuelCastReq{})
	dt.a.msgs, dt.b.msgs = nil, nil

	dt.wait(10 * time.Second)
	dt.guess(dt.a, "PILOT") // no green, nothing moves
	none(t, dt.a, protocol.ServerDuelEyes)
	dt.guess(dt.a, "CLAMP")
	one[protocol.DuelEyes](t, dt.a, protocol.ServerDuelEyes)
	if ev := one[protocol.DuelCast](t, dt.a, protocol.ServerDuelCast); ev.Ability != classes.Wise {
		t.Fatalf("event %+v", ev)
	}
	none(t, dt.b, protocol.ServerDuelCast)
	// The timer starts again
	if next := me.eyes[0].next; !next.Equal(dt.clock.Add(EyeInterval)) {
		t.Fatalf("next move %v", next)
	}
}

func TestMagicMissile(t *testing.T) {
	t.Run("lands if they don't guess", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
		me, them := dt.side(dt.pa), dt.side(dt.pb)
		me.energy, them.energy = 6, 7
		dt.cast(dt.a, slotMissile, protocol.DuelCastReq{})
		if s := dt.state(dt.b); len(s.You.MissilesMs) != 1 || s.You.MissilesMs[0] != 15000 {
			t.Fatalf("missile %+v", s.You)
		}
		dt.wait(14 * time.Second)
		if them.rows != 3 {
			t.Fatal("landed early")
		}
		dt.b.msgs = nil
		dt.wait(time.Second)
		if them.rows != 2 || them.energy != 2 || len(them.missiles) != 0 {
			t.Fatalf("rows %d energy %d", them.rows, them.energy)
		}
		if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); ev.Kind != protocol.CastLanded {
			t.Fatalf("event %+v", ev)
		}
	})
	t.Run("a guess beats it", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
		dt.side(dt.pa).energy = 6
		them := dt.side(dt.pb)
		dt.cast(dt.a, slotMissile, protocol.DuelCastReq{})
		dt.a.msgs = nil
		dt.wait(10 * time.Second)
		dt.guess(dt.b, "PILOT")
		if ev := one[protocol.DuelCast](t, dt.a, protocol.ServerDuelCast); ev.Kind != protocol.CastFizzled || !ev.ByYou {
			t.Fatalf("event %+v", ev)
		}
		dt.wait(10 * time.Second)
		if them.rows != 3 {
			t.Fatal("it landed anyway")
		}
	})
	t.Run("never takes the last row", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
		dt.side(dt.pa).energy = 6
		them := dt.side(dt.pb)
		dt.guess(dt.b, "PILOT")
		dt.guess(dt.b, "PILOT")
		them.energy = 3
		dt.cast(dt.a, slotMissile, protocol.DuelCastReq{})
		dt.wait(MissileFlight)
		if them.rows != 3 || them.energy != 0 {
			t.Fatalf("rows %d energy %d", them.rows, them.energy)
		}
	})
	t.Run("a shield stops it when it lands", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
		dt.side(dt.pa).energy = 6
		them := dt.side(dt.pb)
		them.energy = 7
		dt.cast(dt.a, slotMissile, protocol.DuelCastReq{})
		if them.shield {
			t.Fatal("setup")
		}
		them.shield = true
		dt.wait(MissileFlight)
		if them.rows != 3 || them.energy != 7 || them.shield {
			t.Fatalf("rows %d energy %d shield %v", them.rows, them.energy, them.shield)
		}
	})
}

func TestIllusion(t *testing.T) {
	t.Run("solving it frees them", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
		me, them := dt.side(dt.pa), dt.side(dt.pb)
		me.energy = 16
		dt.cast(dt.a, slotIllusion, protocol.DuelCastReq{})
		if s := one[protocol.IllusionStart](t, dt.b, protocol.ServerIllusionStart); s.WordLength != 3 || s.MaxGuesses != IllusionGuesses {
			t.Fatalf("start %+v", s)
		}
		dt.a.msgs = nil
		// One at a time
		dt.cast(dt.a, slotIllusion, protocol.DuelCastReq{})
		if me.energy != 8 {
			t.Fatal("a second illusion")
		}
		if res := dt.guess(dt.b, "CLAMP"); !res.Blocked {
			t.Fatal("guessed in the duel while in an illusion")
		}
		dt.IllusionGuess(dt.b, "DOG")
		dt.IllusionGuess(dt.b, "cat")
		dt.flush()
		res := got[protocol.WordleRes](t, dt.b, protocol.ServerIllusionGuess)
		if len(res) != 2 || res[1].Status != protocol.WordleWin {
			t.Fatalf("illusion guesses %+v", res)
		}
		if end := one[protocol.IllusionEnd](t, dt.b, protocol.ServerIllusionEnd); !end.Won || end.EnergyLost != 0 {
			t.Fatalf("end %+v", end)
		}
		if ev := one[protocol.DuelCast](t, dt.a, protocol.ServerDuelCast); ev.Kind != protocol.CastFizzled || ev.Ability != classes.Illusion {
			t.Fatalf("caster told %+v", ev)
		}
		if them.illusion != nil || !dt.guess(dt.b, "CLAMP").Valid {
			t.Fatal("still trapped")
		}
	})
	t.Run("failing it costs energy", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Wizard, classes.Rogue)
		dt.side(dt.pa).energy = 8
		them := dt.side(dt.pb)
		them.energy = 7
		dt.cast(dt.a, slotIllusion, protocol.DuelCastReq{})
		dt.a.msgs = nil
		for range IllusionGuesses {
			dt.IllusionGuess(dt.b, "DOG")
		}
		// Past the end, ignored
		dt.IllusionGuess(dt.b, "CAT")
		dt.flush()
		if end := one[protocol.IllusionEnd](t, dt.b, protocol.ServerIllusionEnd); end.Won || end.Solution != "CAT" || end.EnergyLost != 5 {
			t.Fatalf("end %+v", end)
		}
		if them.energy != 2 || them.illusion != nil {
			t.Fatalf("energy %d", them.energy)
		}
		if ev := one[protocol.DuelCast](t, dt.a, protocol.ServerDuelCast); ev.Kind != protocol.CastLanded {
			t.Fatalf("caster told %+v", ev)
		}
	})
}

func TestReshapeReality(t *testing.T) {
	// The options skip the current word and anything they guessed
	dt := newAbilityDuel(t, classes.Wizard, classes.Rogue, "CRANE", "PILOT", "SLATE", "TRACE", "MOUNT", "BRICK", "GHOST", "FLOUR")
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	dt.guess(dt.b, "PILOT")
	dt.guess(dt.b, "CLAMP")
	me.energy = 12

	dt.cast(dt.a, slotReshape, protocol.DuelCastReq{})
	opts := one[protocol.DuelReshapeOptions](t, dt.a, protocol.ServerDuelReshapeOptions)
	if len(opts.Words) != ReshapeOptions || slices.Contains(opts.Words, "CRANE") || slices.Contains(opts.Words, "PILOT") {
		t.Fatalf("options %v", opts.Words)
	}
	if me.energy != 12 {
		t.Fatal("paid before picking")
	}

	dt.ReshapeReality(dt.a, "CRANE")
	dt.flush()
	if them.word != "CRANE" || me.energy != 12 {
		t.Fatal("picked a word that wasn't offered")
	}

	dt.a.msgs, dt.b.msgs = nil, nil
	dt.ReshapeReality(dt.a, strings.ToLower(opts.Words[0])) // SLATE
	dt.flush()
	if them.word != "SLATE" || me.energy != 0 || me.word != "CRANE" {
		t.Fatalf("their word %s, mine %s, energy %d", them.word, me.word, me.energy)
	}
	want := [][]protocol.WordleColor{
		{protocol.Grey, protocol.Grey, protocol.Yellow, protocol.Grey, protocol.Yellow}, // PILOT vs SLATE
		{protocol.Grey, protocol.Green, protocol.Green, protocol.Grey, protocol.Grey},   // CLAMP vs SLATE
	}
	for _, c := range []*fakeClient{dt.a, dt.b} {
		b := one[protocol.DuelBoard](t, c, protocol.ServerDuelBoard)
		if b.Yours != (c == dt.b) || !slices.EqualFunc(b.Colors, want, slices.Equal) {
			t.Fatalf("board %+v, want %v", b, want)
		}
	}
	// Relearned without being paid: L and A green now
	if !them.greens[1] || !them.greens[2] || them.greens[0] {
		t.Fatalf("greens %v", them.greens)
	}

	// Can't pick again without casting again
	me.energy = 12
	dt.ReshapeReality(dt.a, opts.Words[1])
	dt.flush()
	if them.word != "SLATE" {
		t.Fatal("picked twice")
	}

	dt.guess(dt.b, "SLATE")
	if end := one[protocol.DuelEnd](t, dt.a, protocol.ServerDuelEnd); end.Solution != "CRANE" || end.Outcome != protocol.DuelLose {
		t.Fatalf("a's end %+v", end)
	}
	if end := one[protocol.DuelEnd](t, dt.b, protocol.ServerDuelEnd); end.Solution != "SLATE" {
		t.Fatalf("b's end %+v", end)
	}
}

func TestReshapeRealityIsBlockedByAShield(t *testing.T) {
	dt := newAbilityDuel(t, classes.Wizard, classes.Rogue, "SLATE", "TRACE", "MOUNT", "BRICK", "GHOST")
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	me.energy = 12
	dt.cast(dt.a, slotReshape, protocol.DuelCastReq{})
	opts := one[protocol.DuelReshapeOptions](t, dt.a, protocol.ServerDuelReshapeOptions)
	them.shield = true
	dt.ReshapeReality(dt.a, opts.Words[0])
	dt.flush()
	if them.word != "CRANE" || them.shield || me.energy != 0 {
		t.Fatalf("word %s shield %v energy %d", them.word, them.shield, me.energy)
	}
	none(t, dt.b, protocol.ServerDuelBoard)
}
