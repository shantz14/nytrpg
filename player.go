package main

import (
	"fmt"
	"time"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Player struct {
	id int

	conn *websocket.Conn

}

type PlayerData struct {
	ID int `msgpack:"id"`
	Pos Vector2D `msgpack:"pos"`
	
	// True when this is the data of the player being sent to
	Me bool `msgpack:"me"`
}

func handleWS(h *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Connection failed at Upgrader: ", err)
		return
	}

	fmt.Println("New connection coming from: ", conn.RemoteAddr())

	// Add new conn to set
	h.numPlayers += 1
	newPlayer := Player{id: h.numPlayers, conn: conn}
	h.players[&newPlayer] = true

	// Add pos to gamestate
	h.state.Players[newPlayer.id] = &PlayerData{ID: newPlayer.id, Pos: Vector2D{X: 0, Y: 0}}

	go newPlayer.handlePlayer(h)
}

func (p *Player) handlePlayer(h *Hub) {
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

			// Assign their id
			inData.ID = p.id

			h.in <- inData
		}

		// Write
		var outData GameState
		outData = *h.state

		// Set players own data ME flag
		// TODO: Consolidate these 2 id values
		outData.Players[p.id].Me = true
		// SOMEWHERE all players are being set to Me
		// TODO: Find where this is happening and remove this loop
		for k := range outData.Players {
			if k != p.id {
				outData.Players[k].Me = false
			}
		}

		outBuff, err := msgpack.Marshal(outData)
		if err := p.conn.WriteMessage(websocket.BinaryMessage, outBuff); err != nil {
			fmt.Println("Error writing msg to player: ", err)
		}
	}

}





