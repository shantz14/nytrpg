package main

import (
	"nytrpg/resources"

)

type GameState struct {
	Players map[int]*PlayerData `msgpack:"players"`
	Unregister int `msgpack:"unregister"`
}

type Hub struct {
	players map[*Player]bool
	state *GameState
	in chan PlayerData
	unregister chan *Player
	resourceManager *resources.ResourceManager
	db *Connection
}

func newHub() *Hub {
	return &Hub {
		players: make(map[*Player]bool),
		state: &GameState {
			Players: make(map[int]*PlayerData),
			Unregister: -999,
		},
		in: make(chan PlayerData),
		unregister: make(chan *Player),
		resourceManager: resources.NewResourceManager(),
		db: newConnection(),
	}
}

func (h *Hub) run() {
	go h.resourceManager.LoadResources()

	for {

		select {
		case updateData := <-h.in:
			// Update server state
			h.state.Players[updateData.ID].Pos.X = updateData.Pos.X
			h.state.Players[updateData.ID].Pos.Y = updateData.Pos.Y

		case player := <-h.unregister:
			// Unregister 
			h.state.Unregister = player.id

			delete(h.state.Players, player.id)
			delete(h.players, player)

		}


	}
}


