// Package game owns the shared world: who is connected and where they are.
package game

import (
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"nytrpg/internal/netconn"
	"nytrpg/internal/protocol"
)

const maxChatLen = 200

type Hub struct {
	// Only touched by the hub goroutine
	players map[*netconn.Session]*protocol.PlayerSnap
	dirty   bool

	in         chan move
	register   chan *netconn.Session
	unregister chan *netconn.Session
	chatIn     chan protocol.ChatMsg
	closeAll   chan struct{}

	// Shared with HTTP handlers and sessions
	onlineMu sync.Mutex
	online   map[int]bool
}

func NewHub() *Hub {
	return &Hub{
		players:    make(map[*netconn.Session]*protocol.PlayerSnap),
		in:         make(chan move),
		register:   make(chan *netconn.Session),
		unregister: make(chan *netconn.Session),
		chatIn:     make(chan protocol.ChatMsg),
		closeAll:   make(chan struct{}),
		online:     make(map[int]bool),
	}
}

type move struct {
	s   *netconn.Session
	pos protocol.Vec
}

func (h *Hub) RegisterHandlers(r *netconn.Router) {
	r.Handle(protocol.ClientMove, func(s *netconn.Session, data msgpack.RawMessage) {
		var pos protocol.Vec
		if err := msgpack.Unmarshal(data, &pos); err != nil {
			return
		}
		h.in <- move{s, pos}
	})
	r.Handle(protocol.ClientChat, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.ChatReq
		if err := msgpack.Unmarshal(data, &req); err != nil {
			return
		}
		if !s.Allow("chat", 1, 3) {
			return
		}
		// Who said it comes from the connection, never the client
		h.chatIn <- protocol.ChatMsg{ID: s.PlayerID, Msg: req.Msg}
	})
}

func (h *Hub) Join(s *netconn.Session)  { h.register <- s }
func (h *Hub) Leave(s *netconn.Session) { h.unregister <- s }

// Disconnects everyone, for shutting down
func (h *Hub) CloseAll() { h.closeAll <- struct{}{} }

func (h *Hub) Run() {
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()

	for {
		h.step(ticker.C)
	}
}

// Handles one event. A panic is logged instead of taking the server down.
func (h *Hub) step(tick <-chan time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in hub: %v\n%s", r, debug.Stack())
		}
	}()

	select {
	case m := <-h.in:
		if p, ok := h.players[m.s]; ok {
			p.Pos = m.pos
			h.dirty = true
		}

	case s := <-h.unregister:
		delete(h.players, s)
		h.ReleaseOnline(s.PlayerID)
		h.dirty = true

	case s := <-h.register:
		h.players[s] = &protocol.PlayerSnap{ID: s.PlayerID, Username: s.Username}
		s.SendMsg(protocol.ServerWelcome, protocol.Welcome{PlayerID: s.PlayerID, Username: s.Username})
		h.dirty = true

	case chat := <-h.chatIn:
		h.broadcastChat(chat)

	case <-h.closeAll:
		for s := range h.players {
			go s.CloseGoingAway()
		}

	case <-tick:
		if h.dirty {
			h.broadcastState()
			h.dirty = false
		}
	}
}

// Everyone gets the same snapshot, so it's encoded once
func (h *Hub) broadcastState() {
	snap := protocol.Snapshot{Players: make([]protocol.PlayerSnap, 0, len(h.players))}
	for _, p := range h.players {
		snap.Players = append(snap.Players, *p)
	}
	msg, err := protocol.Encode(protocol.ServerSnapshot, snap)
	if err != nil {
		log.Println("Error encoding snapshot:", err)
		return
	}
	for s := range h.players {
		s.Send(msg)
	}
}

// Runs on the hub goroutine, so it must never block
func (h *Hub) broadcastChat(chat protocol.ChatMsg) {
	chat.Msg = strings.TrimSpace(chat.Msg)
	if chat.Msg == "" {
		return
	}
	if runes := []rune(chat.Msg); len(runes) > maxChatLen {
		chat.Msg = string(runes[:maxChatLen])
	}

	msg, err := protocol.Encode(protocol.ServerChat, chat)
	if err != nil {
		log.Println("Error encoding chat:", err)
		return
	}
	for s := range h.players {
		s.Send(msg)
	}
}

// Marks a player online. Returns false if they already were.
func (h *Hub) ClaimOnline(id int) bool {
	h.onlineMu.Lock()
	defer h.onlineMu.Unlock()
	if h.online[id] {
		return false
	}
	h.online[id] = true
	return true
}

func (h *Hub) ReleaseOnline(id int) {
	h.onlineMu.Lock()
	defer h.onlineMu.Unlock()
	delete(h.online, id)
}

func (h *Hub) IsOnline(id int) bool {
	h.onlineMu.Lock()
	defer h.onlineMu.Unlock()
	return h.online[id]
}
