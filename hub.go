package main

import (
	/*
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	*/
)


type Hub struct {
	players map[*Player]bool

}

func newHub() *Hub {
	return &Hub {
		make(map[*Player]bool),
	}
}

func (h *Hub) run() {
}

func (h *Hub) handlePlayer(p *Player) {
	/*updateInterval := time.Second / 30

	// Read
	for {
		messageType, buff, err := p.conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
		} else {
			msgpack.Unmarshal()
		}
	}

	// Write
	*/
}
