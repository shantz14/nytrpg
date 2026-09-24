package game

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"nytrpg/internal/protocol"
)

type fakeClient struct {
	msgs []decoded
}

type decoded struct {
	t    protocol.ServerMsg
	data msgpack.RawMessage
}

func (c *fakeClient) Send(msg []byte) bool {
	t, data, err := protocol.Decode(msg)
	if err != nil {
		panic(err)
	}
	c.msgs = append(c.msgs, decoded{protocol.ServerMsg(t), data})
	return true
}

// Returns and forgets the messages of type t
func (c *fakeClient) take(t protocol.ServerMsg) []msgpack.RawMessage {
	var out []msgpack.RawMessage
	rest := c.msgs[:0]
	for _, m := range c.msgs {
		if m.t == t {
			out = append(out, m.data)
		} else {
			rest = append(rest, m)
		}
	}
	c.msgs = rest
	return out
}

func (c *fakeClient) updates(t *testing.T) []protocol.WorldUpdate {
	var out []protocol.WorldUpdate
	for _, raw := range c.take(protocol.ServerWorld) {
		var u protocol.WorldUpdate
		if err := msgpack.Unmarshal(raw, &u); err != nil {
			t.Fatal(err)
		}
		out = append(out, u)
	}
	return out
}

type testWorld struct {
	*World
	clock time.Time
}

func newTestWorld() *testWorld {
	m := &protocol.WorldMap{Width: 5000, Height: 5000, Spawn: protocol.Vec{X: 500, Y: 500}}
	tw := &testWorld{World: NewWorld(m), clock: time.Unix(1000, 0)}
	tw.now = func() time.Time { return tw.clock }
	tw.rng = rand.New(rand.NewSource(1))
	return tw
}

// Runs queued commands, then one tick
func (tw *testWorld) tickNow() {
	tw.flush()
	tw.step()
}

func (tw *testWorld) flush() {
	for {
		select {
		case cmd := <-tw.cmds:
			cmd()
		default:
			return
		}
	}
}

func (tw *testWorld) advance(d time.Duration) { tw.clock = tw.clock.Add(d) }

// Joins a player and puts them at pos
func (tw *testWorld) join(id int, pos protocol.Vec) (*fakeClient, *player) {
	c := &fakeClient{}
	tw.Join(c, id, "p", protocol.CharacterInfo{ID: id, Name: fmt.Sprintf("char%d", id), Class: "wizard"})
	tw.flush()
	p := tw.players[c]
	p.ent.Pos = pos
	tw.grid.moved(p.ent)
	c.msgs = nil
	return c, p
}

func TestWelcome(t *testing.T) {
	tw := newTestWorld()
	c := &fakeClient{}
	tw.Join(c, 7, "alice", protocol.CharacterInfo{ID: 3, Slot: 1, Name: "Merlin", Class: "wizard"})
	tw.flush()
	raw := c.take(protocol.ServerWelcome)
	if len(raw) != 1 {
		t.Fatalf("want 1 welcome, got %d", len(raw))
	}
	var w protocol.Welcome
	msgpack.Unmarshal(raw[0], &w)
	if w.PlayerID != 7 || w.Username != "alice" || w.EntityID == 0 || w.Map.Width != 5000 {
		t.Fatalf("bad welcome %+v", w)
	}
	if d := w.Pos.X - 500; d < -spawnSpread || d > spawnSpread {
		t.Fatalf("spawned too far from spawn point: %+v", w.Pos)
	}
	if w.Character != (protocol.CharacterInfo{ID: 3, Slot: 1, Name: "Merlin", Class: "wizard"}) {
		t.Fatalf("welcome should say which character you are: %+v", w.Character)
	}
	if len(w.Classes) != 4 {
		t.Fatalf("welcome should list every class: %+v", w.Classes)
	}
}

func TestSpawnMoveIdleDespawn(t *testing.T) {
	tw := newTestWorld()
	a, pa := tw.join(1, protocol.Vec{X: 100, Y: 100})
	b, pb := tw.join(2, protocol.Vec{X: 200, Y: 200})

	tw.tickNow()
	ua := a.updates(t)
	if len(ua) != 1 || len(ua[0].Spawn) != 1 || ua[0].Spawn[0].ID != pb.ent.ID {
		t.Fatalf("a should see b spawn once: %+v", ua)
	}
	if s := ua[0].Spawn[0]; s.Name != "p" || s.Char != "char2" || s.Class != "wizard" {
		t.Fatalf("spawn should carry the username, character name and class: %+v", s)
	}
	ub := b.updates(t)
	if len(ub) != 1 || len(ub[0].Spawn) != 1 || ub[0].Spawn[0].ID != pa.ent.ID {
		t.Fatalf("b should see a spawn once, never itself: %+v", ub)
	}

	// Nothing changed: nothing sent
	tw.tickNow()
	if len(a.msgs)+len(b.msgs) != 0 {
		t.Fatalf("idle tick sent %d messages", len(a.msgs)+len(b.msgs))
	}

	// b moves: a gets a move, b gets nothing about itself
	tw.advance(100 * time.Millisecond)
	tw.Move(b, protocol.Vec{X: 230, Y: 200})
	tw.tickNow()
	ua = a.updates(t)
	if len(ua) != 1 || len(ua[0].Move) != 1 || ua[0].Move[0] != (protocol.EntityMove{ID: pb.ent.ID, X: 230, Y: 200}) {
		t.Fatalf("a should see b move: %+v", ua)
	}
	if len(b.msgs) != 0 {
		t.Fatalf("b was told about its own move")
	}

	// b leaves: a gets a despawn
	tw.Leave(b)
	tw.tickNow()
	ua = a.updates(t)
	if len(ua) != 1 || len(ua[0].Despawn) != 1 || ua[0].Despawn[0] != pb.ent.ID {
		t.Fatalf("a should see b despawn: %+v", ua)
	}
	if tw.IsOnline(2) {
		t.Fatal("b still online after leaving")
	}
}

func TestInterestManagement(t *testing.T) {
	tw := newTestWorld()
	a, _ := tw.join(1, protocol.Vec{X: 100, Y: 100})
	// Three cells away, out of view
	far, pfar := tw.join(2, protocol.Vec{X: 100 + 3*cellSize, Y: 100})

	tw.tickNow()
	if len(a.msgs) != 0 || len(far.msgs) != 0 {
		t.Fatal("players out of view were told about each other")
	}

	// Walk far into view, one allowed step at a time
	pos := pfar.ent.Pos
	for pos.X > 100+cellSize {
		tw.advance(time.Second)
		pos.X -= 400
		tw.Move(far, pos)
		tw.tickNow()
	}
	spawned := false
	for _, u := range a.updates(t) {
		for _, s := range u.Spawn {
			spawned = spawned || s.ID == pfar.ent.ID
		}
	}
	if !spawned {
		t.Fatal("a never saw far come into view")
	}

	// And back out
	for pos.X < 100+3*cellSize {
		tw.advance(time.Second)
		pos.X += 400
		tw.Move(far, pos)
		tw.tickNow()
	}
	despawned := false
	for _, u := range a.updates(t) {
		for _, id := range u.Despawn {
			despawned = despawned || id == pfar.ent.ID
		}
	}
	if !despawned {
		t.Fatal("a never saw far leave view")
	}
}

func TestMoveValidation(t *testing.T) {
	tw := newTestWorld()
	c, p := tw.join(1, protocol.Vec{X: 1000, Y: 1000})
	start := p.ent.Pos

	// Teleport: rejected with a correction to where they really are
	tw.advance(50 * time.Millisecond)
	tw.Move(c, protocol.Vec{X: 4000, Y: 4000})
	tw.flush()
	if p.ent.Pos != start {
		t.Fatalf("teleport accepted: %+v", p.ent.Pos)
	}
	corr := c.take(protocol.ServerCorrection)
	var v protocol.Vec
	if len(corr) != 1 || msgpack.Unmarshal(corr[0], &v) != nil || v != start {
		t.Fatalf("want correction to %+v, got %v", start, corr)
	}

	// Moving at full speed for a few seconds is fine, even diagonally rounded
	pos := start
	speed := MoveSpeed
	for i := 0; i < 60; i++ {
		tw.advance(time.Second / 20)
		pos.X += int32(speed / 20)
		tw.Move(c, pos)
		tw.flush()
	}
	if p.ent.Pos != pos || len(c.take(protocol.ServerCorrection)) != 0 {
		t.Fatalf("full speed movement was corrected")
	}

	// Moving at double speed gets caught once the saved up budget runs out
	for i := 0; i < 60; i++ {
		tw.advance(time.Second / 20)
		pos.X += int32(2 * speed / 20)
		tw.Move(c, pos)
		tw.flush()
	}
	if len(c.take(protocol.ServerCorrection)) == 0 {
		t.Fatal("double speed never corrected")
	}

	// Out of bounds is rejected
	tw.advance(time.Second)
	tw.Move(c, protocol.Vec{X: -10, Y: p.ent.Pos.Y})
	tw.flush()
	if p.ent.Pos.X < 0 || len(c.take(protocol.ServerCorrection)) != 1 {
		t.Fatal("out of bounds move accepted")
	}
}

func TestChatHeardByEveryone(t *testing.T) {
	tw := newTestWorld()
	a, pa := tw.join(1, protocol.Vec{X: 100, Y: 100})
	near, _ := tw.join(2, protocol.Vec{X: 300, Y: 100})
	// Chat is global: players out of view hear it too (it used to be nearby only)
	far, _ := tw.join(3, protocol.Vec{X: 4000, Y: 4000})
	tw.tickNow()
	a.msgs, near.msgs, far.msgs = nil, nil, nil

	tw.Chat(a, "  hello  ")
	tw.Chat(a, "   ")
	tw.flush()
	// Named, so the far player's chat log can say who it was
	want := protocol.ChatMsg{ID: pa.ent.ID, Msg: "hello", Name: "p", Char: "char1", Class: "wizard"}
	for _, c := range []*fakeClient{a, near, far} {
		chats := c.take(protocol.ServerChat)
		var m protocol.ChatMsg
		if len(chats) != 1 || msgpack.Unmarshal(chats[0], &m) != nil || m != want {
			t.Fatalf("chat not delivered right: %d chats, %+v", len(chats), m)
		}
	}
}

func TestChatIsCapped(t *testing.T) {
	tw := newTestWorld()
	a, _ := tw.join(1, protocol.Vec{X: 100, Y: 100})
	tw.Chat(a, strings.Repeat("é", maxChatLen+50))
	tw.flush()
	chats := a.take(protocol.ServerChat)
	var m protocol.ChatMsg
	if len(chats) != 1 || msgpack.Unmarshal(chats[0], &m) != nil || len([]rune(m.Msg)) != maxChatLen {
		t.Fatalf("want one %d rune chat, got %d chats of %d runes", maxChatLen, len(chats), len([]rune(m.Msg)))
	}
}

func TestSystemsRunEachTick(t *testing.T) {
	tw := newTestWorld()
	runs := 0
	tw.AddSystem(func(w *World) { runs++ })
	tw.tickNow()
	tw.tickNow()
	if runs != 2 {
		t.Fatalf("system ran %d times", runs)
	}
}

func TestPanicInCommandIsRecovered(t *testing.T) {
	tw := newTestWorld()
	tw.safely(func() { panic("boom") })
	if tw.Stats.Panics.Load() != 1 {
		t.Fatal("panic not counted")
	}
}

// 200 players on the town map, a fifth of them moving each tick. Spread is how
// much of the map they're scattered over, 1 = the whole map.
func benchmarkTick(b *testing.B, spread float64) {
	tw := newTestWorld()
	tw.Map.Width, tw.Map.Height = 4690, 4690
	area := int32(4690 * spread)
	rng := rand.New(rand.NewSource(1))
	var clients []*fakeClient
	for i := 0; i < 200; i++ {
		c, _ := tw.join(i, protocol.Vec{X: rng.Int31n(area), Y: rng.Int31n(area)})
		clients = append(clients, c)
	}
	tw.tickNow()
	startBytes := tw.Stats.BytesOut.Load()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tw.advance(time.Second / TickRate)
		for j := 0; j < 40; j++ {
			c := clients[rng.Intn(len(clients))]
			p := tw.players[c].ent.Pos
			tw.Move(c, protocol.Vec{X: clamp(p.X+rng.Int31n(41)-20, 0, area), Y: clamp(p.Y+rng.Int31n(41)-20, 0, area)})
		}
		tw.tickNow()
		for _, c := range clients {
			c.msgs = c.msgs[:0]
		}
	}
	perClientPerSec := float64(tw.Stats.BytesOut.Load()-startBytes) / float64(b.N) / 200 * TickRate
	b.ReportMetric(perClientPerSec/1024, "KB/s/client")
}

func BenchmarkWorldTickSpread(b *testing.B)  { benchmarkTick(b, 1) }
func BenchmarkWorldTickCrowded(b *testing.B) { benchmarkTick(b, 0.2) }

func TestInRange(t *testing.T) {
	m := &protocol.WorldMap{
		Width: 5000, Height: 5000, Spawn: protocol.Vec{X: 500, Y: 500},
		Interactables: []protocol.Interactable{
			{ID: "board", Pos: protocol.Vec{X: 1000, Y: 1000}, W: 100, H: 100, Range: 300},
			{ID: "sign", Pos: protocol.Vec{X: 4000, Y: 4000}, W: 100, H: 100},
		},
	}
	w := NewWorld(m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	c := &fakeClient{}
	w.Join(c, 1, "p", protocol.CharacterInfo{ID: 1, Name: "c", Class: "knight"})
	at := func(x, y int32) {
		w.Query(func(w *World) {
			p := w.players[c]
			p.ent.Pos = protocol.Vec{X: x, Y: y}
			w.grid.moved(p.ent)
		})
	}

	at(1050, 1050) // on it
	if !w.InRange(c, "board") {
		t.Fatal("on the board should be in range")
	}
	at(1100+300, 1050) // exactly at range from the right edge
	if !w.InRange(c, "board") {
		t.Fatal("at the range limit should be in range")
	}
	at(1100+301, 1100+1) // just past it, diagonally
	if w.InRange(c, "board") {
		t.Fatal("past the range should be out of range")
	}
	if !w.InRange(c, "sign") {
		t.Fatal("range 0 means anywhere")
	}
	if w.InRange(c, "nope") {
		t.Fatal("unknown interactable should be out of range")
	}
	if w.InRange(&fakeClient{}, "board") {
		t.Fatal("unknown client should be out of range")
	}
}
