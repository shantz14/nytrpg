// Package testkit runs a real server in-process for tests, and talks to it the
// way a browser does: HTTP for accounts, a websocket for the game.
//
//	ts := testkit.NewServer(t)
//	alice := ts.Connect(t, "alice")        // signs up, logs in, dials, reads the welcome
//	alice.Send(t, protocol.ClientChat, protocol.ChatReq{Msg: "hi"})
//	chat := testkit.Expect[protocol.ChatMsg](t, alice, protocol.ServerChat)
//
// Everything uses a temporary database and is cleaned up when the test ends.
package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/crypto/bcrypt"

	"nytrpg/internal/auth"
	"nytrpg/internal/config"
	"nytrpg/internal/protocol"
	"nytrpg/internal/server"
)

func init() {
	// Real password hashing is deliberately slow, and very slow under -race
	auth.BcryptCost = bcrypt.MinCost
}

// How long Expect and friends wait before failing the test
const Timeout = 3 * time.Second

const password = "pw"

// A running server
type Server struct {
	*server.Server
	HTTP *httptest.Server
	URL  string
}

func NewServer(t testing.TB) *Server {
	t.Helper()
	cfg := config.Config{
		DBPath:    filepath.Join(t.TempDir(), "test.db"),
		StaticDir: t.TempDir(),
		JWTSecret: []byte("test-secret"),
	}
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		h.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})
	return &Server{Server: srv, HTTP: h, URL: h.URL}
}

var nameCounter atomic.Int64

// A username no other test uses
func UniqueName(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, nameCounter.Add(1))
}

// POSTs JSON and decodes the JSON reply into out (if not nil). Returns the status.
func (s *Server) PostJSON(t testing.TB, path string, body, out any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(s.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decoding %s reply: %v", path, err)
		}
	}
	return resp.StatusCode
}

// GETs and decodes the JSON reply into out (if not nil). Returns the status.
func (s *Server) GetJSON(t testing.TB, path string, out any) int {
	t.Helper()
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decoding %s reply: %v", path, err)
		}
	}
	return resp.StatusCode
}

type Account struct {
	ID       int
	Username string
	Token    string
}

// Signs up (if needed) and logs in
func (s *Server) Login(t testing.TB, username string) Account {
	t.Helper()
	creds := map[string]string{"username": username, "password": password}
	s.PostJSON(t, "/signup", creds, nil)
	var res struct {
		ValidUser bool   `json:"validUser"`
		Id        int    `json:"id"`
		Jwt       string `json:"jwt"`
		Username  string `json:"username"`
	}
	if code := s.PostJSON(t, "/login", creds, &res); code != http.StatusOK || !res.ValidUser {
		t.Fatalf("login %s failed: %d %+v", username, code, res)
	}
	return Account{ID: res.Id, Username: res.Username, Token: res.Jwt}
}

// Logs in as username and connects, see Dial
func (s *Server) Connect(t testing.TB, username string) *Client {
	t.Helper()
	return s.Dial(t, s.Login(t, username))
}

// Opens a websocket as the account and waits for the welcome
func (s *Server) Dial(t testing.TB, acct Account) *Client {
	t.Helper()
	c, status, err := s.DialRaw(acct.Token)
	if err != nil {
		t.Fatalf("dial as %s: %d %v", acct.Username, status, err)
	}
	c.Account = acct
	c.Welcome = Expect[protocol.Welcome](t, c, protocol.ServerWelcome)
	c.Pos = c.Welcome.Pos
	return c
}

// Opens a websocket with any token, for testing auth. Doesn't wait for anything.
func (s *Server) DialRaw(token string) (*Client, int, error) {
	wsURL := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws?token=" + url.QueryEscape(token)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	if err != nil {
		return nil, status, err
	}
	c := &Client{conn: conn, msgs: make(chan Msg, 4096), closed: make(chan error, 1)}
	go c.read()
	return c, status, nil
}

// One message from the server, payload still encoded
type Msg struct {
	Type protocol.ServerMsg
	Data msgpack.RawMessage
}

// A websocket connection, as the game client
type Client struct {
	Account Account
	Welcome protocol.Welcome
	// Where this player is, as far as the test has moved it (see WalkTo)
	Pos protocol.Vec

	conn   *websocket.Conn
	msgs   chan Msg
	closed chan error
}

func (c *Client) read() {
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			c.closed <- err
			close(c.msgs)
			return
		}
		t, data, err := protocol.Decode(raw)
		if err != nil {
			continue
		}
		c.msgs <- Msg{protocol.ServerMsg(t), data}
	}
}

func (c *Client) Send(t testing.TB, msgType protocol.ClientMsg, payload any) {
	t.Helper()
	msg, err := protocol.EncodeClient(msgType, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatalf("send: %v", err)
	}
}

// Writes raw bytes, for testing bad input
func (c *Client) SendRaw(t testing.TB, data []byte) error {
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *Client) Close() { c.conn.Close() }

// Waits for the server to end the connection and returns why
func (c *Client) WaitClosed(t testing.TB) error {
	t.Helper()
	select {
	case err := <-c.closed:
		return err
	case <-time.After(Timeout):
		t.Fatal("connection wasn't closed")
		return nil
	}
}

// Waits for the next message of msgType, skipping others, and decodes it
func Expect[T any](t testing.TB, c *Client, msgType protocol.ServerMsg) T {
	t.Helper()
	var out T
	m, ok := c.next(msgType, Timeout)
	if !ok {
		t.Fatalf("no message of type %d within %s", msgType, Timeout)
	}
	if err := msgpack.Unmarshal(m.Data, &out); err != nil {
		t.Fatalf("decoding message %d: %v", msgType, err)
	}
	return out
}

// Fails if a message of msgType arrives within d
func (c *Client) ExpectNone(t testing.TB, msgType protocol.ServerMsg, d time.Duration) {
	t.Helper()
	if m, ok := c.next(msgType, d); ok {
		t.Fatalf("unexpected message of type %d: %x", msgType, []byte(m.Data))
	}
}

// Counts the messages of msgType that arrive within d
func (c *Client) Count(msgType protocol.ServerMsg, d time.Duration) int {
	n := 0
	deadline := time.After(d)
	for {
		if _, ok := c.nextWithDeadline(msgType, deadline); !ok {
			return n
		}
		n++
	}
}

func (c *Client) next(msgType protocol.ServerMsg, d time.Duration) (Msg, bool) {
	return c.nextWithDeadline(msgType, time.After(d))
}

// What a client knows about the world, built from world updates
type View struct {
	Pos     map[protocol.EntityID]protocol.Vec
	Names   map[protocol.EntityID]string
	Despawn map[protocol.EntityID]bool
}

// Applies world updates for d, or until done(view) returns true
func (c *Client) WatchWorld(d time.Duration, done func(View) bool) View {
	v := View{
		Pos:     map[protocol.EntityID]protocol.Vec{},
		Names:   map[protocol.EntityID]string{},
		Despawn: map[protocol.EntityID]bool{},
	}
	deadline := time.After(d)
	for {
		if done != nil && done(v) {
			return v
		}
		m, ok := c.nextWithDeadline(protocol.ServerWorld, deadline)
		if !ok {
			return v
		}
		var u protocol.WorldUpdate
		if msgpack.Unmarshal(m.Data, &u) != nil {
			continue
		}
		for _, s := range u.Spawn {
			v.Pos[s.ID] = s.Pos
			v.Names[s.ID] = s.Name
			delete(v.Despawn, s.ID)
		}
		for _, mv := range u.Move {
			v.Pos[mv.ID] = protocol.Vec{X: mv.X, Y: mv.Y}
		}
		for _, id := range u.Despawn {
			delete(v.Pos, id)
			v.Despawn[id] = true
		}
	}
}

func (c *Client) nextWithDeadline(msgType protocol.ServerMsg, deadline <-chan time.Time) (Msg, bool) {
	for {
		select {
		case m, ok := <-c.msgs:
			if !ok {
				return Msg{}, false
			}
			if m.Type == msgType {
				return m, true
			}
		case <-deadline:
			return Msg{}, false
		}
	}
}

// Walks toward target in legal steps (just under speed px/s, 20 moves a
// second like the real client) and returns once there. Use it to put a player
// somewhere without being corrected.
func (c *Client) WalkTo(t testing.TB, target protocol.Vec, speed float64) {
	t.Helper()
	const rate = 20
	step := speed / rate * 0.9
	for c.Pos != target {
		dx, dy := float64(target.X-c.Pos.X), float64(target.Y-c.Pos.Y)
		dist := math.Hypot(dx, dy)
		if dist <= step {
			c.Pos = target
		} else {
			c.Pos.X += int32(dx / dist * step)
			c.Pos.Y += int32(dy / dist * step)
		}
		c.Send(t, protocol.ClientMove, c.Pos)
		time.Sleep(time.Second / rate)
	}
}
