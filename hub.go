package main

import (
)

type GameState struct {
	Players map[int]*PlayerData `msgpack:"players"`
}

type Hub struct {
	players map[*Player]bool

	numPlayers int

	state *GameState

	in chan PlayerData
}

func newHub() *Hub {
	return &Hub {
		players: make(map[*Player]bool),
		numPlayers: 0,
		state: &GameState {
			Players: make(map[int]*PlayerData),
		},
		in: make(chan PlayerData),
	}
}

func (h *Hub) run() {
	for {
		inData := <-h.in

		// Update server state
		h.state.Players[inData.ID].Pos.X = inData.Pos.X
		h.state.Players[inData.ID].Pos.Y = inData.Pos.Y
		h.state.Players[inData.ID].ID = inData.ID
	}
}


