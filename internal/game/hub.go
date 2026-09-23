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
	players map[*netconn.Session]bool
	state   map[int]*protocol.PlayerData
	dirty   bool

	in         chan protocol.PlayerData
	register   chan *netconn.Session
	unregister chan *netconn.Session
	chatIn     chan protocol.Chat
	closeAll   chan struct{}

	// Shared with HTTP handlers and sessions
	onlineMu sync.Mutex
	online   map[int]bool
}

func NewHub() *Hub {
	return &Hub{
		players:    make(map[*netconn.Session]bool),
		state:      make(map[int]*protocol.PlayerData),
		in:         make(chan protocol.PlayerData),
		register:   make(chan *netconn.Session),
		unregister: make(chan *netconn.Session),
		chatIn:     make(chan protocol.Chat),
		closeAll:   make(chan struct{}),
		online:     make(map[int]bool),
	}
}

func (h *Hub) RegisterHandlers(r *netconn.Router) {
	r.Handle(protocol.ClientUpdatePos, func(s *netconn.Session, data []byte) {
		var pos protocol.PlayerData
		if err := msgpack.Unmarshal(data, &pos); err != nil {
			log.Println("Client data could not be asserted as type PlayerData.")
			return
		}
		pos.ID = s.PlayerID
		h.in <- pos
	})
	r.Handle(protocol.ClientRecChat, func(s *netconn.Session, data []byte) {
		var chat protocol.Chat
		if err := msgpack.Unmarshal(data, &chat); err != nil {
			log.Println("Client data could not be asserted as type Chat.")
			return
		}
		if !s.Allow("chat", 1, 3) {
			return
		}
		// Never trust who the client says sent it
		chat.ID = s.PlayerID
		h.chatIn <- chat
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
	case update := <-h.in:
		if p, ok := h.state[update.ID]; ok {
			p.Pos = update.Pos
			h.dirty = true
		}

	case s := <-h.unregister:
		delete(h.state, s.PlayerID)
		delete(h.players, s)
		h.ReleaseOnline(s.PlayerID)
		h.dirty = true

	case s := <-h.register:
		h.players[s] = true
		h.state[s.PlayerID] = &protocol.PlayerData{ID: s.PlayerID, Username: s.Username}
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

func (h *Hub) broadcastState() {
	for s := range h.players {
		state := protocol.GameState{Players: make(map[int]*protocol.PlayerData, len(h.state))}
		for id, p := range h.state {
			copied := *p
			copied.Me = id == s.PlayerID
			state.Players[id] = &copied
		}
		s.SendMsg(protocol.ServerUpdatePos, state)
	}
}

// Runs on the hub goroutine, so it must never block
func (h *Hub) broadcastChat(chat protocol.Chat) {
	chat.Msg = strings.TrimSpace(chat.Msg)
	if chat.Msg == "" {
		return
	}
	if runes := []rune(chat.Msg); len(runes) > maxChatLen {
		chat.Msg = string(runes[:maxChatLen])
	}

	msg, err := protocol.Encode(protocol.ServerSendChat, chat)
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
