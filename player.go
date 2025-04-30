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
	h.currentID += 1
	newPlayer := Player{id: h.currentID, conn: conn}
	h.players[&newPlayer] = true

	// Add pos to gamestate
	h.state.Players[newPlayer.id] = &PlayerData{ID: newPlayer.id, Pos: Vector2D{X: 0, Y: 0}, Me: false}

	go newPlayer.handlePlayer(h)
}

func (p *Player) handlePlayer(h *Hub) {
	defer func() {
		h.unregister <- p
		p.conn.Close()
	}()

	updateInterval := time.Second / 30

	for range time.Tick(updateInterval) {
		// Read
		_, inBuff, err := p.conn.ReadMessage() // No use for msg type yet
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				fmt.Println("Player disconnected")
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Println("Error reading msg from player: ", err)
			}
			break

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
		outData.Players = make(map[int]*PlayerData)

		// TODO: I think just making one new PlayerData and swapping just that reference in outData would be better
		// Failed attempts: 1
		for id, player := range h.state.Players {
			var newPlayer PlayerData
			newPlayer.ID = id
			newPlayer.Pos = player.Pos

			if (id == p.id) {
				newPlayer.Me = true
			} else {
				newPlayer.Me = false
			}

			outData.Players[id] = &newPlayer
		}


		outData.Unregister = h.state.Unregister

		outBuff, err := msgpack.Marshal(outData)
		if err != nil {
			fmt.Println("Error encoding: ", err)
		}

		if err := p.conn.WriteMessage(websocket.BinaryMessage, outBuff); err != nil {
			fmt.Println("Error writing msg to player: ", err)
		}
	}

}





