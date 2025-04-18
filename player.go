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
	loc Vector2D

	conn *websocket.Conn

	in chan[]byte

	out chan[]byte
}

func handleWS(h *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Connection failed at Upgrader: ", err)
		return
	}

	fmt.Println("New connection coming from: ", conn.RemoteAddr())

	// Add new conn to set
	newPlayer := Player{loc: Vector2D{x: 0, y:0}, conn: conn}
	h.players[&newPlayer] = true

	go h.handlePlayer(&newPlayer)
}





