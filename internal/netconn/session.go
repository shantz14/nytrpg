// Package netconn wraps a player's websocket: one reader, one writer, and a
// router that hands incoming messages to whichever feature registered for them.
package netconn

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/time/rate"

	"nytrpg/internal/protocol"
)

const (
	// Messages waiting to be written. A client this far behind is disconnected.
	sendQueueSize = 64
	// Largest message a client may send
	maxMessageSize = 4096

	writeWait  = 5 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second

	// Every message a client sends
	msgRate  = 60
	msgBurst = 120
	// Messages dropped in a row before the client is kicked for flooding
	maxDropped = 100
)

// Counters across all sessions, for monitoring
var Stats struct {
	Open            atomic.Int64 // connections open right now
	Total           atomic.Int64 // connections ever opened
	SlowClientKicks atomic.Int64
	FloodKicks      atomic.Int64
	RateLimited     atomic.Int64 // messages dropped by the per-session rate limit
	BadMessages     atomic.Int64 // undecodable or unknown message types
	HandlerPanics   atomic.Int64
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Session struct {
	PlayerID int
	Username string
	// The character being played
	CharacterID int

	conn      *websocket.Conn
	log       *slog.Logger
	send      chan []byte
	closed    chan struct{}
	closeOnce sync.Once

	msgLimit *rate.Limiter
	// Per feature limits, see Allow
	limitsMu sync.Mutex
	limits   map[string]*rate.Limiter
}

func newSession(conn *websocket.Conn, playerID int, username string, characterID int) *Session {
	return &Session{
		PlayerID:    playerID,
		Username:    username,
		CharacterID: characterID,
		conn:        conn,
		log:         slog.With("player", playerID, "user", username, "character", characterID, "addr", conn.RemoteAddr().String()),
		send:        make(chan []byte, sendQueueSize),
		closed:      make(chan struct{}),
		msgLimit:    rate.NewLimiter(msgRate, msgBurst),
		limits:      make(map[string]*rate.Limiter),
	}
}

// Queues an encoded message. Never blocks. A client whose queue is full is too
// slow to keep up and gets disconnected.
func (s *Session) Send(msg []byte) bool {
	select {
	case <-s.closed:
		return false
	default:
	}
	select {
	case s.send <- msg:
		return true
	default:
		s.log.Warn("client can't keep up, disconnecting", "queued", len(s.send))
		Stats.SlowClientKicks.Add(1)
		s.Close()
		return false
	}
}

// Encodes and queues a message
func (s *Session) SendMsg(t protocol.ServerMsg, data any) bool {
	msg, err := protocol.Encode(t, data)
	if err != nil {
		s.log.Error("encoding message", "type", t, "err", err)
		return false
	}
	return s.Send(msg)
}

// Rate limits one kind of action for this player, e.g. Allow("chat", 1, 3) allows
// a burst of 3 then one per second. The first call for a key sets its limits.
func (s *Session) Allow(key string, perSecond float64, burst int) bool {
	s.limitsMu.Lock()
	l, ok := s.limits[key]
	if !ok {
		l = rate.NewLimiter(rate.Limit(perSecond), burst)
		s.limits[key] = l
	}
	s.limitsMu.Unlock()
	return l.Allow()
}

// Disconnects. Safe to call more than once, from any goroutine.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.conn.Close()
	})
}

// Close code telling the client this player connected from somewhere else.
// Clients must not reconnect after it, or two tabs would keep kicking each other.
const CloseReplaced = 4001

// Tells the client the server is going away, then disconnects
func (s *Session) CloseGoingAway() {
	s.CloseWith(websocket.CloseGoingAway, "server shutting down")
}

// Sends a close frame with a code and reason, then disconnects
func (s *Session) CloseWith(code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	// Control frames may be written alongside the writer goroutine
	s.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	s.Close()
}

// Upgrades to a websocket and runs the session until the connection ends.
// onJoin runs before any message is read, onLeave after the connection is gone.
func Serve(w http.ResponseWriter, r *http.Request, playerID int, username string, characterID int, router *Router, onJoin, onLeave func(*Session)) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	s := newSession(conn, playerID, username, characterID)
	s.log.Info("connected")
	Stats.Open.Add(1)
	Stats.Total.Add(1)
	defer Stats.Open.Add(-1)

	onJoin(s)
	defer onLeave(s)
	s.run(router)
	return nil
}

// Runs the session until the connection ends
func (s *Session) run(router *Router) {
	defer s.Close()
	go s.writeLoop()
	s.readLoop(router)
}

func (s *Session) readLoop(router *Router) {
	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	dropped := 0
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.log.Info("disconnected")
			} else {
				s.log.Info("disconnected", "err", err)
			}
			return
		}
		// Any message proves the client is alive
		s.conn.SetReadDeadline(time.Now().Add(pongWait))

		if !s.msgLimit.Allow() {
			Stats.RateLimited.Add(1)
			dropped++
			if dropped > maxDropped {
				s.log.Warn("flooding, disconnecting")
				Stats.FloodKicks.Add(1)
				return
			}
			continue
		}
		dropped = 0

		if !s.dispatch(router, data) {
			return
		}
	}
}

// Runs one handler. A panic is logged and ends this session instead of the server.
func (s *Session) dispatch(router *Router, data []byte) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			Stats.HandlerPanics.Add(1)
			s.log.Error("panic handling message", "panic", r, "stack", string(debug.Stack()))
			ok = false
		}
	}()
	router.dispatch(s, data)
	return true
}

// The only goroutine that writes data messages to the connection
func (s *Session) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		s.Close()
	}()

	for {
		select {
		case <-s.closed:
			return
		case msg := <-s.send:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				s.log.Info("write failed", "err", err)
				return
			}
		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Handles one message type. data is the still encoded payload, unmarshal it
// into the type the message carries.
type Handler func(s *Session, data msgpack.RawMessage)

type Router struct {
	handlers map[protocol.ClientMsg]Handler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[protocol.ClientMsg]Handler)}
}

func (r *Router) Handle(t protocol.ClientMsg, h Handler) {
	if _, ok := r.handlers[t]; ok {
		panic("netconn: duplicate handler for message type")
	}
	r.handlers[t] = h
}

func (r *Router) dispatch(s *Session, raw []byte) {
	t, data, err := protocol.Decode(raw)
	if err != nil {
		Stats.BadMessages.Add(1)
		s.log.Debug("bad message", "err", err)
		return
	}
	h, ok := r.handlers[protocol.ClientMsg(t)]
	if !ok {
		Stats.BadMessages.Add(1)
		s.log.Debug("unknown message type", "type", t)
		return
	}
	h(s, data)
}
