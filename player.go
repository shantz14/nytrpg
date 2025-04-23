package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Player struct {
	id int

	conn *websocket.Conn

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

	go h.handlePlayer(&newPlayer)
}





