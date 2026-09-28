package game

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"nytrpg/internal/classes"
	"nytrpg/internal/oracle"
	"nytrpg/internal/protocol"
)

const (
	slotPickpocket, slotCheat, slotFeint, slotUnderTheirNose, slotConfuse = 0, 1, 2, 3, 4
	slotMinorPrayer, slotMend, slotPurify, slotMajorPrayer, slotDivine    = 0, 1, 2, 3, 4
)

var (
	G, Y, X = protocol.Green, protocol.Yellow, protocol.Grey
	// A pattern for Feint and Under Their Nose
	fakeColors = []protocol.WordleColor{G, Y, X, X, G}
)

// Runs the next command queued from off the world goroutine, like a prayer's
// answer
func (dt *duelTest) awaitCmd() {
	dt.t.Helper()
	select {
	case cmd := <-dt.cmds:
		cmd()
	case <-time.After(2 * time.Second):
		dt.t.Fatal("nothing came back to the world")
	}
}

// Runs commands coming back to the world until ok
func (dt *duelTest) until(ok func() bool) {
	dt.t.Helper()
	deadline := time.After(2 * time.Second)
	for !ok() {
		select {
		case cmd := <-dt.cmds:
			cmd()
		case <-deadline:
			dt.t.Fatal("timed out")
		}
	}
}

func TestPickpocketRevealsAYellowLetterToTheCaster(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Rogue)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	me.energy = 10
	dt.guess(dt.b, "ACORN") // on CRANE: A C _ R N yellow, O grey
	dt.a.msgs, dt.b.msgs = nil, nil

	dt.cast(dt.a, slotPickpocket, protocol.DuelCastReq{Row: 0, Col: 2})
	none(t, dt.a, protocol.ServerDuelPickpocket)
	if me.energy != 10 {
		t.Fatal("pickpocketed a grey letter")
	}
	dt.cast(dt.a, slotPickpocket, protocol.DuelCastReq{Row: 0, Col: 3})
	if p := one[protocol.DuelPickpocket](t, dt.a, protocol.ServerDuelPickpocket); p.Letter != "R" || p.Row != 0 || p.Col != 3 {
		t.Fatalf("pickpocket %+v", p)
	}
	none(t, dt.b, protocol.ServerDuelPickpocket)
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); ev.Ability != classes.Pickpocket || ev.ByYou {
		t.Fatalf("victim told %+v", ev)
	}

	them.shield = true
	dt.cast(dt.a, slotPickpocket, protocol.DuelCastReq{Row: 0, Col: 0})
	none(t, dt.a, protocol.ServerDuelPickpocket)
	if them.shield || me.energy != 6 {
		t.Fatal("the shield should take it, and it's still paid for")
	}
}

func TestPickpocketGoesByTheColorsTheCasterSaw(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Rogue)
	dt.side(dt.pa).energy = 10
	dt.side(dt.pb).energy = 10
	// b feints: a sees PILOT's P as yellow though it's grey
	dt.cast(dt.b, slotFeint, protocol.DuelCastReq{Colors: []protocol.WordleColor{Y, X, X, X, X}})
	dt.guess(dt.b, "PILOT")
	dt.cast(dt.a, slotPickpocket, protocol.DuelCastReq{Row: 0, Col: 0})
	if p := one[protocol.DuelPickpocket](t, dt.a, protocol.ServerDuelPickpocket); p.Letter != "P" {
		t.Fatalf("pickpocket %+v", p)
	}
}

func TestCheatTakesOneNonWord(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Knight)
	me := dt.side(dt.pa)
	me.energy = 10
	if res := dt.guess(dt.a, "XXXXX"); res.Valid {
		t.Fatal("XXXXX isn't a word")
	}
	dt.cast(dt.a, slotCheat, protocol.DuelCastReq{})
	if s := dt.state(dt.a); !s.You.CheatReady {
		t.Fatalf("state %+v", s.You)
	}
	if s := dt.state(dt.b); s.Them.CheatReady {
		t.Fatal("the opponent can see it's ready")
	}
	dt.cast(dt.a, slotCheat, protocol.DuelCastReq{})
	if me.energy != 5 {
		t.Fatal("cheated twice at once")
	}
	if res := dt.guess(dt.a, "XXXXX"); !res.Valid {
		t.Fatal("cheat didn't take a non-word")
	}
	if res := dt.guess(dt.a, "XXXXX"); res.Valid {
		t.Fatal("cheat lasts one guess")
	}
}

func TestFeintShowsTheOpponentPickedColors(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Knight)
	me := dt.side(dt.pa)
	me.energy = 20
	for _, bad := range [][]protocol.WordleColor{{G, G, G, G, G}, {G, Y}, {G, Y, X, X, 7}} {
		dt.cast(dt.a, slotFeint, protocol.DuelCastReq{Colors: bad})
	}
	if me.energy != 20 {
		t.Fatal("paid for a bad pattern")
	}
	dt.cast(dt.a, slotFeint, protocol.DuelCastReq{Colors: fakeColors})
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); ev.Ability != classes.Feint {
		t.Fatalf("the opponent should know a feint is coming: %+v", ev)
	}
	res := dt.guess(dt.a, "PILOT")
	if !slices.Equal(res.Colors, []protocol.WordleColor{X, X, X, X, X}) {
		t.Fatalf("a should see the real colors, got %v", res.Colors)
	}
	if og := one[protocol.DuelOpponentGuess](t, dt.b, protocol.ServerDuelOpponentGuess); !slices.Equal(og.Colors, fakeColors) {
		t.Fatalf("b saw %v", og.Colors)
	}
	// One guess only
	dt.guess(dt.a, "PILOT")
	if og := one[protocol.DuelOpponentGuess](t, dt.b, protocol.ServerDuelOpponentGuess); slices.Equal(og.Colors, fakeColors) {
		t.Fatal("the feint lasted")
	}

	// It can't hide a solve
	dt.cast(dt.a, slotFeint, protocol.DuelCastReq{Colors: fakeColors})
	dt.guess(dt.a, "CRANE")
	if end := one[protocol.DuelEnd](t, dt.b, protocol.ServerDuelEnd); end.Outcome != protocol.DuelLose {
		t.Fatalf("end %+v", end)
	}
}

func TestUnderTheirNoseIsSecret(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Rogue)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	me.energy = 21
	dt.cast(dt.a, slotUnderTheirNose, protocol.DuelCastReq{Colors: fakeColors})
	none(t, dt.b, protocol.ServerDuelCast)
	if ev := one[protocol.DuelCast](t, dt.a, protocol.ServerDuelCast); ev.Ability != classes.UnderTheirNose || !ev.ByYou {
		t.Fatalf("caster told %+v", ev)
	}

	res := dt.guess(dt.b, "PILOT")
	if !slices.Equal(res.Colors, fakeColors) {
		t.Fatalf("b should see the fake colors, got %v", res.Colors)
	}
	if og := one[protocol.DuelOpponentGuess](t, dt.a, protocol.ServerDuelOpponentGuess); !slices.Equal(og.Colors, []protocol.WordleColor{X, X, X, X, X}) {
		t.Fatalf("a sees the real colors, got %v", og.Colors)
	}
	if them.energy != 0 {
		t.Fatal("energy comes from the real colors, not the fake greens")
	}

	// It can't hide a solve
	dt.cast(dt.a, slotUnderTheirNose, protocol.DuelCastReq{Colors: fakeColors})
	if res := dt.guess(dt.b, "CRANE"); res.Status != protocol.WordleWin || !solves(res.Colors) {
		t.Fatalf("solve %+v", res)
	}
}

func TestUnderTheirNoseIsCaughtByAShield(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Rogue)
	dt.side(dt.pa).energy = 7
	them := dt.side(dt.pb)
	them.shield = true
	dt.cast(dt.a, slotUnderTheirNose, protocol.DuelCastReq{Colors: fakeColors})
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); !ev.Blocked || ev.Ability != classes.UnderTheirNose {
		t.Fatalf("victim told %+v", ev)
	}
	if them.falseNext != nil {
		t.Fatal("it got through the shield")
	}
}

func isPermutation(keys string) bool {
	b := []byte(keys)
	slices.Sort(b)
	return string(b) == "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

func TestConfuseScramblesTheNextTwoGuesses(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Knight)
	dt.side(dt.pa).energy = 10
	dt.cast(dt.a, slotConfuse, protocol.DuelCastReq{})
	s := dt.state(dt.b)
	if !isPermutation(s.You.Keymap) || s.You.Keymap == "ABCDEFGHIJKLMNOPQRSTUVWXYZ" || s.You.ScrambledGuesses != 2 {
		t.Fatalf("victim state %+v", s.You)
	}
	if o := dt.state(dt.a).Them; o.Keymap != "" || o.ScrambledGuesses != 2 {
		t.Fatalf("the caster sees it's scrambled, not how: %+v", o)
	}
	dt.guess(dt.b, "PILOT")
	if s := dt.state(dt.b); s.You.ScrambledGuesses != 1 || s.You.Keymap == "" {
		t.Fatalf("after 1 guess %+v", s.You)
	}
	dt.guess(dt.b, "PILOT")
	if s := dt.state(dt.b); s.You.ScrambledGuesses != 0 || s.You.Keymap != "" {
		t.Fatalf("after 2 guesses %+v", s.You)
	}
}

func TestSneakySwapsTwoKeysOnANewGreen(t *testing.T) {
	dt := newAbilityDuel(t, classes.Rogue, classes.Knight)
	them := dt.side(dt.pb)
	dt.guess(dt.a, "PILOT")
	if them.keymap != nil {
		t.Fatal("swapped without a green")
	}
	dt.guess(dt.a, "CLAMP")
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); ev.Ability != classes.Sneaky || ev.Kind != protocol.CastTriggered {
		t.Fatalf("event %+v", ev)
	}
	moved := 0
	for i, k := range them.keymap {
		if k != byte('A'+i) {
			moved++
		}
	}
	if moved != 2 || them.scrambledGuesses != ScrambleGuesses {
		t.Fatalf("keymap %s for %d guesses", them.keymap, them.scrambledGuesses)
	}
}

func TestDivineWillSilences(t *testing.T) {
	dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
	them := dt.side(dt.pb)
	them.energy = 10
	dt.guess(dt.a, "CLAMP")
	if s := dt.state(dt.b); s.You.SilencedMs != 10000 {
		t.Fatalf("silence %+v", s.You)
	}
	dt.cast(dt.b, slotShields, protocol.DuelCastReq{})
	if them.shield || them.energy != 10 {
		t.Fatal("cast while silenced")
	}
	dt.wait(DivineWillSilence)
	if s := dt.state(dt.b); s.You.SilencedMs != 0 {
		t.Fatalf("the end of the silence is sent %+v", s.You)
	}
	dt.cast(dt.b, slotShields, protocol.DuelCastReq{})
	if !them.shield {
		t.Fatal("still silenced")
	}
}

func TestMendTakesBackAGuess(t *testing.T) {
	dt := newAbilityDuel(t, classes.Cleric, classes.Wizard)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	for _, g := range []string{"PILOT", "ACORN", "SLATE"} {
		dt.guess(dt.a, g)
	}
	if !me.out() {
		t.Fatal("setup: a should be out")
	}
	me.destroyed = map[tile]bool{{0, 1}: true, {2, 3}: true}
	them.eyes = []*eye{{at: tile{2, 0}, on: true}}
	me.energy = 3
	dt.a.msgs, dt.b.msgs = nil, nil

	dt.cast(dt.a, slotMend, protocol.DuelCastReq{Row: 5})
	none(t, dt.a, protocol.ServerDuelGuessRemoved)
	dt.cast(dt.a, slotMend, protocol.DuelCastReq{Row: 0})
	if r := one[protocol.DuelGuessRemoved](t, dt.a, protocol.ServerDuelGuessRemoved); !r.Yours || r.Row != 0 {
		t.Fatalf("a told %+v", r)
	}
	if r := one[protocol.DuelGuessRemoved](t, dt.b, protocol.ServerDuelGuessRemoved); r.Yours || r.Row != 0 {
		t.Fatalf("b told %+v", r)
	}
	if len(me.guesses) != 2 || me.guesses[0].word != "ACORN" || me.out() {
		t.Fatalf("guesses %+v", me.guesses)
	}
	if !maps(me.destroyed, map[tile]bool{{1, 3}: true}) {
		t.Fatalf("destroyed letters should move up with their row: %v", me.destroyed)
	}
	if e := them.eyes[0]; !e.on || e.at != (tile{1, 0}) {
		t.Fatalf("the eye should follow its letter: %+v", e)
	}
	if res := dt.guess(dt.a, "PILOT"); !res.Valid {
		t.Fatal("a should have a row again")
	}
}

func maps(a, b map[tile]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func TestPurifyClearsEveryCurse(t *testing.T) {
	dt := newAbilityDuel(t, classes.Cleric, classes.Wizard)
	me, them := dt.side(dt.pa), dt.side(dt.pb)
	dt.guess(dt.a, "PILOT")
	now := dt.clock
	me.stunnedUntil = now.Add(time.Minute)
	me.keymap, me.scrambledGuesses = []byte("BACDEFGHIJKLMNOPQRSTUVWXYZ"), 2
	me.missiles = []time.Time{now.Add(time.Second)}
	me.falseNext = fakeColors
	me.illusion = &illusionGame{word: "CAT"}
	me.destroyed[tile{0, 0}] = true
	them.eyes = []*eye{{at: tile{0, 1}, on: true}}
	me.energy = 4
	dt.a.msgs = nil

	dt.cast(dt.a, slotPurify, protocol.DuelCastReq{})
	if !me.stunnedUntil.IsZero() || me.keymap != nil || me.scrambledGuesses != 0 || me.missiles != nil || me.falseNext != nil || me.illusion != nil || them.eyes != nil {
		t.Fatalf("still cursed: %+v", me.sideEffects)
	}
	if !me.destroyed[tile{0, 0}] {
		t.Fatal("what's done stays done")
	}
	if end := one[protocol.IllusionEnd](t, dt.a, protocol.ServerIllusionEnd); !end.Purified {
		t.Fatalf("illusion end %+v", end)
	}
	if e := one[protocol.DuelEyes](t, dt.b, protocol.ServerDuelEyes); len(e.Tiles) != 0 {
		t.Fatalf("the wizard's eyes should close: %+v", e)
	}

	// Not while silenced
	me.energy = 4
	me.silencedUntil = dt.clock.Add(time.Minute)
	dt.cast(dt.a, slotPurify, protocol.DuelCastReq{})
	if me.energy != 4 {
		t.Fatal("purified while silenced")
	}
}

func TestDivineIntervention(t *testing.T) {
	intervene := func(t *testing.T, fate protocol.DivineFate, words ...string) *duelTest {
		t.Helper()
		dt := newAbilityDuel(t, classes.Cleric, classes.Knight, words...)
		dt.fate = func() protocol.DivineFate { return fate }
		dt.guess(dt.a, "PILOT")
		dt.guess(dt.b, "ACORN")
		dt.side(dt.pa).energy = 13
		dt.side(dt.pb).energy = 4
		dt.a.msgs, dt.b.msgs = nil, nil
		dt.cast(dt.a, slotDivine, protocol.DuelCastReq{})
		for _, c := range []*fakeClient{dt.a, dt.b} {
			if dv := one[protocol.DuelDivine](t, c, protocol.ServerDuelDivine); dv.Fate != fate || dv.Banner != divineBanners[fate] {
				t.Fatalf("divine %+v", dv)
			}
		}
		return dt
	}

	t.Run("clean slate", func(t *testing.T) {
		dt := intervene(t, protocol.FateCleanSlate)
		for _, s := range []*duelSide{dt.side(dt.pa), dt.side(dt.pb)} {
			if len(s.guesses) != 0 || s.energy != 0 || s.rows != 3 || s.word != "CRANE" {
				t.Fatalf("side %+v", s)
			}
		}
	})
	t.Run("new word", func(t *testing.T) {
		// PILOT and ACORN are guessed, SLATE is the first word free
		dt := intervene(t, protocol.FateNewWord, "CRANE", "PILOT", "ACORN", "SLATE")
		for _, c := range []*fakeClient{dt.a, dt.b} {
			if dt.side(dt.pa).word != "SLATE" || dt.side(dt.pb).word != "SLATE" {
				t.Fatal("both should have the new word")
			}
			boards := got[protocol.DuelBoard](t, c, protocol.ServerDuelBoard)
			if len(boards) != 2 || boards[0].Yours == boards[1].Yours {
				t.Fatalf("both boards should be recolored: %+v", boards)
			}
		}
		// PILOT on SLATE: L and T yellow
		if c := dt.side(dt.pa).guesses[0].colors; !slices.Equal(c, []protocol.WordleColor{X, X, Y, X, Y}) {
			t.Fatalf("PILOT scored %v", c)
		}
	})
	t.Run("sudden death", func(t *testing.T) {
		dt := intervene(t, protocol.FateSuddenDeath)
		if s := dt.state(dt.b); s.DeadlineMs != 120000 {
			t.Fatalf("deadline %d", s.DeadlineMs)
		}
		dt.wait(SuddenDeath)
		if end := one[protocol.DuelEnd](t, dt.b, protocol.ServerDuelEnd); end.Outcome != protocol.DuelDraw || end.Reason != protocol.DuelTimeUp {
			t.Fatalf("end %+v", end)
		}
	})
	t.Run("revelation", func(t *testing.T) {
		dt := intervene(t, protocol.FateRevelation)
		ra := one[protocol.DuelReveal](t, dt.a, protocol.ServerDuelReveal)
		rb := one[protocol.DuelReveal](t, dt.b, protocol.ServerDuelReveal)
		if ra != rb || ra.Letter != "CRANE"[ra.Col:ra.Col+1] {
			t.Fatalf("reveals %+v %+v", ra, rb)
		}
	})
	t.Run("fortune", func(t *testing.T) {
		dt := intervene(t, protocol.FateFortune)
		if dt.side(dt.pa).energy != 4 || dt.side(dt.pb).energy != 3 {
			t.Fatalf("energy %d %d", dt.side(dt.pa).energy, dt.side(dt.pb).energy)
		}
	})
}

// Records prayers and answers them when told to
type heldOracle struct {
	prayers chan oracle.Prayer
	release chan error
}

func (o heldOracle) Pray(ctx context.Context, p oracle.Prayer) (oracle.Answer, error) {
	o.prayers <- p
	if err := <-o.release; err != nil {
		return oracle.Answer{}, err
	}
	return oracle.Answer{God: oracle.Gods[1], Text: "a riddle"}, nil
}

func newHeldOracle() heldOracle {
	return heldOracle{prayers: make(chan oracle.Prayer, 8), release: make(chan error, 8)}
}

func TestPrayersAreAnsweredByAGod(t *testing.T) {
	dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
	o := newHeldOracle()
	dt.Oracle = o
	me := dt.side(dt.pa)
	me.greens = []bool{true, true, false, true, true}
	me.energy = 8

	dt.cast(dt.a, slotMinorPrayer, protocol.DuelCastReq{})
	if p := one[protocol.DuelPrayer](t, dt.a, protocol.ServerDuelPrayer); !p.Pending || p.Kind != protocol.PrayerMinor {
		t.Fatalf("pending %+v", p)
	}
	if ev := one[protocol.DuelCast](t, dt.b, protocol.ServerDuelCast); ev.Ability != classes.MinorPrayer {
		t.Fatalf("the opponent sees the prayer %+v", ev)
	}
	p := <-o.prayers
	if p.Kind != protocol.PrayerMinor || p.Word != "CRANE" || p.Pos != 2 {
		t.Fatalf("prayed %+v: only the letter they haven't found in place", p)
	}
	o.release <- nil
	dt.awaitCmd()
	ans := one[protocol.DuelPrayer](t, dt.a, protocol.ServerDuelPrayer)
	if ans.Pending || ans.Failed || ans.God != oracle.Gods[1].Name || ans.GodTitle == "" || ans.GodColor == "" || ans.Text != "a riddle" {
		t.Fatalf("answer %+v", ans)
	}
	none(t, dt.b, protocol.ServerDuelPrayer)
	if me.energy != 6 {
		t.Fatalf("energy %d", me.energy)
	}

	// Major prayers are about the whole word
	dt.cast(dt.a, slotMajorPrayer, protocol.DuelCastReq{})
	if p := <-o.prayers; p.Kind != protocol.PrayerMajor || p.Word != "CRANE" {
		t.Fatalf("prayed %+v", p)
	}
	o.release <- nil
	dt.awaitCmd()
}

func TestUnansweredPrayersAreRefunded(t *testing.T) {
	t.Run("the gods fail", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
		dt.Oracle = oracle.Fake{Err: errors.New("down")}
		me := dt.side(dt.pa)
		me.energy = 6
		dt.cast(dt.a, slotMajorPrayer, protocol.DuelCastReq{})
		// The fake fails at once, its answer may already be in
		dt.until(func() bool { return me.energy == 6 })
		prayers := got[protocol.DuelPrayer](t, dt.a, protocol.ServerDuelPrayer)
		if len(prayers) != 2 || !prayers[1].Failed || prayers[1].Refunded != 6 || me.energy != 6 {
			t.Fatalf("prayers %+v, energy %d", prayers, me.energy)
		}
		if s := dt.state(dt.a); s.You.Energy != 6 {
			t.Fatal("the refund should be sent")
		}
	})
	t.Run("no gods", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
		me := dt.side(dt.pa)
		me.energy = 2
		dt.cast(dt.a, slotMinorPrayer, protocol.DuelCastReq{})
		prayers := got[protocol.DuelPrayer](t, dt.a, protocol.ServerDuelPrayer)
		if len(prayers) != 2 || !prayers[1].Failed || me.energy != 2 {
			t.Fatalf("prayers %+v, energy %d", prayers, me.energy)
		}
	})
	t.Run("the word changed meanwhile", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
		o := newHeldOracle()
		dt.Oracle = o
		me := dt.side(dt.pa)
		me.energy = 6
		dt.cast(dt.a, slotMajorPrayer, protocol.DuelCastReq{})
		<-o.prayers
		me.word = "SLATE"
		o.release <- nil
		dt.awaitCmd()
		if p := got[protocol.DuelPrayer](t, dt.a, protocol.ServerDuelPrayer); !p[len(p)-1].Failed || me.energy != 6 {
			t.Fatalf("an answer about the old word: %+v", p)
		}
	})
	t.Run("the duel ended meanwhile", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
		o := newHeldOracle()
		dt.Oracle = o
		dt.side(dt.pa).energy = 2
		dt.cast(dt.a, slotMinorPrayer, protocol.DuelCastReq{})
		<-o.prayers
		dt.ForfeitDuel(dt.b)
		dt.flush()
		dt.a.msgs = nil
		o.release <- nil
		dt.awaitCmd()
		if len(dt.a.msgs) != 0 {
			t.Fatal("told about a prayer after the duel")
		}
	})
	t.Run("too many at once", func(t *testing.T) {
		dt := newAbilityDuel(t, classes.Cleric, classes.Knight)
		o := newHeldOracle()
		dt.Oracle = o
		me := dt.side(dt.pa)
		me.energy = 2 * (MaxPrayers + 1)
		for range MaxPrayers + 1 {
			dt.cast(dt.a, slotMinorPrayer, protocol.DuelCastReq{})
		}
		prayers := got[protocol.DuelPrayer](t, dt.a, protocol.ServerDuelPrayer)
		if last := prayers[len(prayers)-1]; !last.Failed || me.energy != 2 {
			t.Fatalf("the prayer over the limit should be refunded: %+v, energy %d", last, me.energy)
		}
		for range MaxPrayers {
			<-o.prayers
			o.release <- nil
			dt.awaitCmd()
		}
	})
}
