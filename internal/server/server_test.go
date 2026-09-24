package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"nytrpg/internal/game"
	"nytrpg/internal/netconn"
	"nytrpg/internal/protocol"
	"nytrpg/internal/testkit"
)

// Integration tests: a real server, real websockets, a temp database. Each test
// gets its own server, so they run in parallel.

func TestWebsocketRequiresValidToken(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	for _, token := range []string{"", "garbage", "a.b.c"} {
		if _, status, err := ts.DialRaw(token, 1); err == nil || status != http.StatusUnauthorized {
			t.Errorf("token %q: want 401, got %d %v", token, status, err)
		}
	}
}

func TestWelcome(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	c := ts.Connect(t, "alice")
	w := c.Welcome
	if w.PlayerID != c.Account.ID || w.Username != "alice" || w.EntityID == 0 {
		t.Fatalf("bad welcome %+v", w)
	}
	if w.Map.Width == 0 || len(w.Map.Interactables) == 0 || w.MoveSpeed != game.MoveSpeed {
		t.Fatalf("welcome is missing the map or rules: %+v", w)
	}
}

func TestNewConnectionReplacesOld(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	acct := ts.Login(t, testkit.UniqueName("p"))
	first := ts.Dial(t, acct)
	second := ts.Dial(t, acct)

	err := first.WaitClosed(t)
	if !websocket.IsCloseError(err, netconn.CloseReplaced) {
		t.Fatalf("first connection should close with %d, got %v", netconn.CloseReplaced, err)
	}
	if second.Welcome.PlayerID != acct.ID {
		t.Fatal("second connection not welcomed")
	}
	// Logging in again is allowed too, it just replaces on connect
	ts.Login(t, acct.Username)
}

func TestPlayersSeeEachOtherMoveAndLeave(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	a := ts.Connect(t, "a")
	b := ts.Connect(t, "b")
	bID := b.Welcome.EntityID

	target := protocol.Vec{X: b.Pos.X + 30, Y: b.Pos.Y}
	b.WalkTo(t, target, game.MoveSpeed)
	view := a.WatchWorld(testkit.Timeout, func(v testkit.View) bool { return v.Pos[bID] == target })
	if view.Pos[bID] != target || view.Names[bID] != "b" {
		t.Fatalf("a should see b arrive at %+v, saw %+v", target, view)
	}
	if _, self := view.Pos[a.Welcome.EntityID]; self {
		t.Fatal("a was sent its own entity")
	}

	b.Close()
	view = a.WatchWorld(testkit.Timeout, func(v testkit.View) bool { return v.Despawn[bID] })
	if !view.Despawn[bID] {
		t.Fatal("a never saw b despawn")
	}
}

func TestTeleportIsCorrected(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	c := ts.Connect(t, "p")
	c.Send(t, protocol.ClientMove, protocol.Vec{X: 4000, Y: 4000})
	corr := testkit.Expect[protocol.Vec](t, c, protocol.ServerCorrection)
	if corr != c.Welcome.Pos {
		t.Fatalf("want correction back to %+v, got %+v", c.Welcome.Pos, corr)
	}
}

func TestChat(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	a := ts.Connect(t, "a")
	b := ts.Connect(t, "b")
	time.Sleep(2 * time.Second / game.TickRate) // a tick, so they know each other

	b.Send(t, protocol.ClientChat, protocol.ChatReq{Msg: "  hello  "})
	msg := testkit.Expect[protocol.ChatMsg](t, a, protocol.ServerChat)
	if msg.ID != b.Welcome.EntityID || msg.Msg != "hello" {
		t.Fatalf("want trimmed chat from b's entity, got %+v", msg)
	}
	// Named by the server from b's session, for the chat log
	if ch := b.Welcome.Character; msg.Name != "b" || msg.Char != ch.Name || msg.Class != ch.Class {
		t.Fatalf("want chat named b / %s / %s, got %+v", ch.Name, ch.Class, msg)
	}
	// The speaker hears themselves
	testkit.Expect[protocol.ChatMsg](t, b, protocol.ServerChat)

	// Flooding is capped by the rate limit: a burst of 3, one already used
	for i := 0; i < 20; i++ {
		b.Send(t, protocol.ClientChat, protocol.ChatReq{Msg: "spam"})
	}
	if got := a.Count(protocol.ServerChat, time.Second); got < 2 || got > 3 {
		t.Fatalf("want 2 more chats through the rate limit, got %d", got)
	}
}

func TestOversizedMessageDisconnects(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	c := ts.Connect(t, "p")
	c.SendRaw(t, make([]byte, 5000))
	if err := c.WaitClosed(t); !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Fatalf("want close 1009, got %v", err)
	}
}

func TestFloodDisconnectsOnlyTheFlooder(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	flooder := ts.Connect(t, "flooder")
	other := ts.Connect(t, "other")
	for i := 0; i < 1000; i++ {
		if flooder.SendRaw(t, []byte{0x80}) != nil {
			break
		}
	}
	flooder.WaitClosed(t)

	other.Send(t, protocol.ClientChat, protocol.ChatReq{Msg: "still here"})
	testkit.Expect[protocol.ChatMsg](t, other, protocol.ServerChat)
}

func TestWordleFlow(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	acct := ts.Login(t, "player")
	c := ts.Dial(t, acct)
	guess := func(word string) protocol.WordleRes {
		c.Send(t, protocol.ClientWordleGuess, protocol.WordleReq{Guess: word})
		return testkit.Expect[protocol.WordleRes](t, c, protocol.ServerWordleResult)
	}

	if guess("CRANE").Valid {
		t.Fatal("guess before starting should be rejected")
	}
	c.Send(t, protocol.ClientWordleStart, nil)
	if r := testkit.Expect[protocol.WordleResume](t, c, protocol.ServerWordleResume); len(r.Guesses) != 0 || r.Played || r.TooFar {
		t.Fatalf("fresh start: %+v", r)
	}

	// A wrong but real word: counted, answer not revealed. Pick one that isn't today's answer.
	wrong := "CRANE"
	if res := guess(wrong); res.Status == protocol.WordleWin {
		wrong = "SLATE"
	} else if !res.Valid || res.Solution != "" || len(res.Colors) != 5 {
		t.Fatalf("first guess: %+v", res)
	}
	if guess("ZZZZZ").Valid {
		t.Fatal("non-word accepted")
	}

	// Reconnecting keeps the guesses and the clock
	c.Close()
	c = ts.Dial(t, acct)
	c.Send(t, protocol.ClientWordleStart, nil)
	r := testkit.Expect[protocol.WordleResume](t, c, protocol.ServerWordleResume)
	if len(r.Guesses) != 1 || r.Guesses[0] != "CRANE" {
		t.Fatalf("resume lost guesses: %+v", r)
	}

	// Use up the guesses
	var last protocol.WordleRes
	for i := 0; i < 4 && last.Status == protocol.WordleInGame; i++ {
		last = guess(wrong)
	}
	if last.Status == protocol.WordleInGame || last.Solution == "" {
		t.Fatalf("game should be over with the answer revealed: %+v", last)
	}
	if guess(wrong).Valid {
		t.Fatal("guess after the game ended was accepted")
	}

	c.Send(t, protocol.ClientWordleStart, nil)
	if r := testkit.Expect[protocol.WordleResume](t, c, protocol.ServerWordleResume); !r.Played {
		t.Fatalf("start after finishing should say played: %+v", r)
	}
}

func TestWordleNeedsToBeNearTheBoard(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	c := ts.Connect(t, "far")
	c.WalkTo(t, protocol.Vec{X: c.Pos.X + 700, Y: c.Pos.Y}, game.MoveSpeed)
	c.Send(t, protocol.ClientWordleStart, nil)
	if r := testkit.Expect[protocol.WordleResume](t, c, protocol.ServerWordleResume); !r.TooFar {
		t.Fatalf("want tooFar, got %+v", r)
	}
	c.ExpectNone(t, protocol.ServerCorrection, 0)
}

func TestLeaderboard(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	var lb struct {
		Date  string `json:"date"`
		Today string `json:"today"`
		Total int    `json:"total"`
		Rows  []any  `json:"rows"`
	}
	if code := ts.GetJSON(t, "/leaderboard", &lb); code != 200 || lb.Date != lb.Today || lb.Rows == nil {
		t.Fatalf("default leaderboard: %d %+v", code, lb)
	}
	for _, q := range []string{"?date=nope", "?page=-1", "?page=x"} {
		if code := ts.GetJSON(t, "/leaderboard"+q, nil); code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", q, code)
		}
	}
}

func TestMetrics(t *testing.T) {
	// Not parallel: metrics are per process
	ts := testkit.NewServer(t)
	ts.Connect(t, "p")
	var vars struct {
		Nytrpg map[string]float64 `json:"nytrpg"`
	}
	if ts.GetJSON(t, "/debug/vars", &vars) != 200 || vars.Nytrpg["connections_open"] < 1 || vars.Nytrpg["players_online"] != 1 {
		t.Fatalf("metrics missing or wrong: %+v", vars.Nytrpg)
	}
	if code := ts.GetJSON(t, "/debug/pprof/", nil); code != http.StatusNotFound {
		t.Fatalf("pprof should be off without DEBUG, got %d", code)
	}
}

func TestShutdownTellsClients(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	c := ts.Connect(t, "p")
	ts.HTTP.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go ts.Shutdown(ctx)
	if err := c.WaitClosed(t); !websocket.IsCloseError(err, websocket.CloseGoingAway) {
		t.Fatalf("want going-away close, got %v", err)
	}
}
