package main

import (
	"nytrpg/resources"
	"sync"
	"time"
)

// Full snapshot of every connected player. Clients replace their view with each one,
// so anyone missing from a snapshot has left.
type GameState struct {
	Players map[int]*PlayerData `msgpack:"players"`
}

type Hub struct {
	// Only touched by the hub goroutine
	players map[*Player]bool
	state *GameState
	dirty bool

	in chan PlayerData
	register chan PlayerConnAndState
	unregister chan *Player
	chatIn chan Chat

	resourceManager *resources.ResourceManager
	db *Connection

	// Shared with HTTP handlers and player goroutines
	onlineMu sync.Mutex
	online map[int]bool

	wordles *WordleSessions
}

func newHub() *Hub {
	rm := resources.NewResourceManager()
	rm.Load()

	return &Hub {
		players: make(map[*Player]bool),
		state: &GameState {
			Players: make(map[int]*PlayerData),
		},
		in: make(chan PlayerData),
		register: make(chan PlayerConnAndState),
		unregister: make(chan *Player),
		chatIn: make(chan Chat),
		resourceManager: rm,
		db: newConnection(),
		online: make(map[int]bool),
		wordles: newWordleSessions(),
	}
}

func (h *Hub) run() {
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop() // always stop it to avoid a goroutine leak

	for {
		select {
		case updateData := <-h.in:
			if p, ok := h.state.Players[updateData.ID]; ok {
				p.Pos.X = updateData.Pos.X
				p.Pos.Y = updateData.Pos.Y
				h.dirty = true
			}

		case player := <-h.unregister:
			delete(h.state.Players, player.id)
			delete(h.players, player)
			h.releaseOnline(player.id)
			h.dirty = true

		case pcas := <-h.register:
			h.players[pcas.playerConn] = true
			h.state.Players[pcas.playerConn.id] = pcas.playerState
			h.dirty = true

		case chat := <-h.chatIn:
			broadcastChat(chat, h)

		case <-ticker.C:
			if h.dirty {
				h.broadcastState()
				h.dirty = false
			}
		}
	}
}

func (h *Hub) broadcastState() {
	// One copy shared by all players, they only read it
	state := deepCopyState(h.state)

	for p := range h.players {
		// Newest state wins: if the player hasn't picked up the last one, replace it
		select {
		case p.stateOut <- state:
		default:
			select {
			case <-p.stateOut:
			default:
			}
			select {
			case p.stateOut <- state:
			default:
			}
		}
	}
}

func deepCopyState(inState *GameState) GameState {
	var state GameState
	state.Players = make(map[int]*PlayerData, len(inState.Players))

	for k, pPtr := range inState.Players {
		state.Players[k] = &PlayerData{}
		*( state.Players[k] ) = *pPtr
	}

	return state
}

// Marks a player online. Returns false if they already were.
func (h *Hub) claimOnline(id int) bool {
	h.onlineMu.Lock()
	defer h.onlineMu.Unlock()
	if h.online[id] {
		return false
	}
	h.online[id] = true
	return true
}

func (h *Hub) releaseOnline(id int) {
	h.onlineMu.Lock()
	defer h.onlineMu.Unlock()
	delete(h.online, id)
}

func (h *Hub) isOnline(id int) bool {
	h.onlineMu.Lock()
	defer h.onlineMu.Unlock()
	return h.online[id]
}
