package netconn

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/time/rate"

	"nytrpg/internal/protocol"
)

// A server that runs sessions with router, and a way to dial it
func serve(t *testing.T, router *Router) (dial func() *websocket.Conn, joined chan *Session) {
	t.Helper()
	joined = make(chan *Session, 16)
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Serve(w, r, 1, "p", router, func(s *Session) { joined <- s }, func(*Session) {})
	}))
	t.Cleanup(h.Close)
	dial = func() *websocket.Conn {
		c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	return dial, joined
}

func send(t *testing.T, c *websocket.Conn, typ protocol.ClientMsg, v any) {
	t.Helper()
	msg, _ := protocol.EncodeClient(typ, v)
	if err := c.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatal(err)
	}
}

func waitClosed(t *testing.T, c *websocket.Conn) error {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			return err
		}
	}
}

func TestRouterDispatchesByType(t *testing.T) {
	router := NewRouter()
	got := make(chan protocol.ChatReq, 1)
	router.Handle(protocol.ClientChat, func(s *Session, data msgpack.RawMessage) {
		var req protocol.ChatReq
		msgpack.Unmarshal(data, &req)
		got <- req
	})
	dial, _ := serve(t, router)
	c := dial()

	before := Stats.BadMessages.Load()
	send(t, c, 99, nil) // nobody handles 99: ignored, connection stays up
	send(t, c, protocol.ClientChat, protocol.ChatReq{Msg: "hi"})
	select {
	case req := <-got:
		if req.Msg != "hi" {
			t.Fatalf("got %+v", req)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler never ran")
	}
	if Stats.BadMessages.Load() != before+1 {
		t.Fatal("unknown message type wasn't counted")
	}
}

func TestDuplicateHandlerPanics(t *testing.T) {
	r := NewRouter()
	r.Handle(protocol.ClientChat, func(*Session, msgpack.RawMessage) {})
	defer func() {
		if recover() == nil {
			t.Fatal("registering a message type twice should panic")
		}
	}()
	r.Handle(protocol.ClientChat, func(*Session, msgpack.RawMessage) {})
}

func TestHandlerPanicClosesOnlyThatSession(t *testing.T) {
	router := NewRouter()
	router.Handle(protocol.ClientChat, func(s *Session, data msgpack.RawMessage) {
		panic("boom")
	})
	ok := make(chan bool, 1)
	router.Handle(protocol.ClientMove, func(s *Session, data msgpack.RawMessage) { ok <- true })
	dial, _ := serve(t, router)
	bad, good := dial(), dial()

	before := Stats.HandlerPanics.Load()
	send(t, bad, protocol.ClientChat, protocol.ChatReq{})
	waitClosed(t, bad)
	if Stats.HandlerPanics.Load() != before+1 {
		t.Fatal("panic wasn't counted")
	}
	send(t, good, protocol.ClientMove, protocol.Vec{})
	select {
	case <-ok:
	case <-time.After(3 * time.Second):
		t.Fatal("other session stopped working after a panic")
	}
}

func TestCloseWithSendsCode(t *testing.T) {
	dial, joined := serve(t, NewRouter())
	c := dial()
	s := <-joined
	s.CloseWith(CloseReplaced, "elsewhere")
	if err := waitClosed(t, c); !websocket.IsCloseError(err, CloseReplaced) {
		t.Fatalf("want close %d, got %v", CloseReplaced, err)
	}
}

func TestSendNeverBlocksAndDropsSlowClients(t *testing.T) {
	// A session whose writer never runs, like a client that stopped reading
	sessions := make(chan *Session, 1)
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		sessions <- newSession(conn, 1, "p")
	}))
	defer h.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s := <-sessions

	before := Stats.SlowClientKicks.Load()
	for i := 0; i < sendQueueSize; i++ {
		if !s.Send([]byte{1}) {
			t.Fatalf("send %d failed before the queue was full", i)
		}
	}
	done := make(chan bool)
	go func() { done <- s.Send([]byte{1}) }()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("send to a full queue succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("Send blocked on a full queue")
	}
	select {
	case <-s.closed:
	default:
		t.Fatal("slow client wasn't disconnected")
	}
	if Stats.SlowClientKicks.Load() != before+1 {
		t.Fatal("kick wasn't counted")
	}
	if s.Send([]byte{1}) {
		t.Fatal("send after close succeeded")
	}
}

func TestAllow(t *testing.T) {
	s := &Session{limits: make(map[string]*rate.Limiter)}
	allowed := 0
	for i := 0; i < 10; i++ {
		if s.Allow("chat", 1, 3) {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("burst of 3 allowed %d", allowed)
	}
	// Separate keys have separate budgets
	if !s.Allow("emote", 1, 1) {
		t.Fatal("a different action was limited by chat")
	}
}
