// Package netconn wraps a player's websocket: one reader, one writer, and a
// router that hands incoming messages to whichever feature registered for them.
package netconn

import (
	"log"
	"net/http"
	"runtime/debug"
	"sync"
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Session struct {
	PlayerID int
	Username string

	conn      *websocket.Conn
	send      chan []byte
	closed    chan struct{}
	closeOnce sync.Once

	msgLimit *rate.Limiter
	// Per feature limits, see Allow
	limitsMu sync.Mutex
	limits   map[string]*rate.Limiter
}

func newSession(conn *websocket.Conn, playerID int, username string) *Session {
	return &Session{
		PlayerID: playerID,
		Username: username,
		conn:     conn,
		send:     make(chan []byte, sendQueueSize),
		closed:   make(chan struct{}),
		msgLimit: rate.NewLimiter(msgRate, msgBurst),
		limits:   make(map[string]*rate.Limiter),
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
		log.Printf("player %d can't keep up, disconnecting", s.PlayerID)
		s.Close()
		return false
	}
}

// Encodes and queues a message
func (s *Session) SendMsg(t protocol.ServerMessageType, data any) bool {
	msg, err := protocol.Encode(t, data)
	if err != nil {
		log.Println("Error encoding message:", err)
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

// Tells the client the server is going away, then disconnects
func (s *Session) CloseGoingAway() {
	msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
	// Control frames may be written alongside the writer goroutine
	s.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	s.Close()
}

// Upgrades to a websocket and runs the session until the connection ends.
// onJoin runs before any message is read, onLeave after the connection is gone.
func Serve(w http.ResponseWriter, r *http.Request, playerID int, username string, router *Router, onJoin, onLeave func(*Session)) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	log.Println("New connection coming from: ", conn.RemoteAddr())

	s := newSession(conn, playerID, username)
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
				log.Printf("player %d disconnected", s.PlayerID)
			} else {
				log.Printf("player %d read error: %v", s.PlayerID, err)
			}
			return
		}
		// Any message proves the client is alive
		s.conn.SetReadDeadline(time.Now().Add(pongWait))

		if !s.msgLimit.Allow() {
			dropped++
			if dropped > maxDropped {
				log.Printf("player %d is flooding, disconnecting", s.PlayerID)
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
			log.Printf("panic handling message from player %d: %v\n%s", s.PlayerID, r, debug.Stack())
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
				log.Printf("player %d write error: %v", s.PlayerID, err)
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

// Handles one message type. data is the message payload.
type Handler func(s *Session, data []byte)

type Router struct {
	handlers map[protocol.ClientMessageType]Handler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[protocol.ClientMessageType]Handler)}
}

func (r *Router) Handle(t protocol.ClientMessageType, h Handler) {
	if _, ok := r.handlers[t]; ok {
		panic("netconn: duplicate handler for message type")
	}
	r.handlers[t] = h
}

func (r *Router) dispatch(s *Session, raw []byte) {
	var msg protocol.ClientMessage
	if err := msgpack.Unmarshal(raw, &msg); err != nil {
		log.Println("Error unpacking envelope data: ", err)
		return
	}
	h, ok := r.handlers[msg.UpdateType]
	if !ok {
		log.Println("No handler for message type", msg.UpdateType)
		return
	}
	h(s, msg.Data)
}
