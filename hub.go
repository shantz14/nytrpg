package main

import (
	"nytrpg/resources"
	"time"
)

type GameState struct {
	Players map[int]*PlayerData `msgpack:"players"`
	Unregister int `msgpack:"unregister"`
}

type Hub struct {
	players map[*Player]bool
	state *GameState
	in chan PlayerData
	register chan PlayerConnAndState
	unregister chan *Player
	resourceManager *resources.ResourceManager
	db *Connection
	chatIn chan Chat
}

func newHub() *Hub {
	return &Hub {
		players: make(map[*Player]bool),
		state: &GameState {
			Players: make(map[int]*PlayerData),
			Unregister: -999,
		},
		in: make(chan PlayerData),
		register: make(chan PlayerConnAndState),
		unregister: make(chan *Player),
		resourceManager: resources.NewResourceManager(),
		db: newConnection(),
		chatIn: make(chan Chat),
	}
}

func (h *Hub) run() {
	go h.resourceManager.LoadResources()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop() // always stop it to avoid a goroutine leak

	for {

		select {
		case updateData := <-h.in:
			h.state.Players[updateData.ID].Pos.X = updateData.Pos.X
			h.state.Players[updateData.ID].Pos.Y = updateData.Pos.Y
			h.broadcastState()

		case player := <-h.unregister:
			h.state.Unregister = player.id
			delete(h.state.Players, player.id)
			delete(h.players, player)
			h.broadcastState()

		case pcas := <-h.register:
			h.players[pcas.playerConn] = true
			h.state.Players[pcas.playerConn.id] = pcas.playerState


		case chat := <-h.chatIn:
			broadcastChat(chat, h)

		// case <- ticker.C:
		}


	}
}

func (h *Hub) broadcastState() {
	state := deepCopyState(h.state)

	for p := range h.players {
		select {
		case p.stateOut <- state:
		default:
		}
	}
}

func deepCopyState(inState *GameState) GameState {
	var state GameState
	state.Players = make(map[int]*PlayerData)

	state.Unregister = inState.Unregister

	for k, pPtr := range inState.Players {
		state.Players[k] = &PlayerData{}
		*( state.Players[k] ) = *pPtr
	}

	return state
}



