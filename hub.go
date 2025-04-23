package main

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

type PlayerData struct {
	ID int `msgpack:"id"`
	Pos Vector2D `msgpack:"pos"`
}

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
	}
}

func (h *Hub) handlePlayer(p *Player) {
	updateInterval := time.Second / 30

	for range time.Tick(updateInterval) {
		// Read
		_, inBuff, err := p.conn.ReadMessage() // No use for msg type yet
		if err != nil {
			fmt.Println("Error reading msg from player: ", err)
		} else {
			var inData PlayerData
			if err := msgpack.Unmarshal(inBuff, &inData); err != nil {
				fmt.Println("Error unpacking data: ", err)
			}

			// Assign their id, where SHOULD THIS be?
			inData.ID = p.id

			// SEND TO IN CHAN
			h.in <- inData
		}

		// Write
		var outData GameState
		outData = *h.state

		outBuff, err := msgpack.Marshal(outData)
		if err := p.conn.WriteMessage(websocket.BinaryMessage, outBuff); err != nil {
			fmt.Println("Error writing msg to player: ", err)
		}
	}

}
