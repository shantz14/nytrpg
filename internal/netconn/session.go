// Package netconn wraps a player's websocket: one reader, one writer, and a
// router that hands incoming messages to whichever feature registered for them.
package netconn

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"

	"nytrpg/internal/protocol"
)

const sendQueueSize = 64

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
}

func newSession(conn *websocket.Conn, playerID int, username string) *Session {
	return &Session{
		PlayerID: playerID,
		Username: username,
		conn:     conn,
		send:     make(chan []byte, sendQueueSize),
		closed:   make(chan struct{}),
	}
}

// Queues an encoded message. Never blocks, returns false if it was dropped.
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

// Disconnects. Safe to call more than once, from any goroutine.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.conn.Close()
	})
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
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Println("Player disconnected")
			} else {
				log.Println("Error reading msg from player: ", err)
			}
			return
		}
		router.dispatch(s, data)
	}
}

// The only goroutine that writes to the connection
func (s *Session) writeLoop() {
	defer s.Close()
	for {
		select {
		case <-s.closed:
			return
		case msg := <-s.send:
			if err := s.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				log.Println("Error writing msg to player: ", err)
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
