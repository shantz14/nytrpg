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
			p.handleMsg(inBuff, h)
		}

		// Write
		var playerData GameState
		playerData.Players = make(map[int]*PlayerData)

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

			playerData.Players[id] = &newPlayer
		}


		playerData.Unregister = h.state.Unregister

		p.send(playerData, ServerUpdatePos)

	}

}

func (p *Player) send(data any, msgType ServerMessageType) {
	var envelope ServerMessage
	envelope.UpdateType = msgType
	var asserted any

	if (msgType == ServerUpdatePos) {
		asserted = data.(GameState)
	} else if (msgType == ServerSendWordle) {
		asserted = data.(WordleRes)
	}

	dataBuff, err := msgpack.Marshal(asserted)
	if err != nil {
		fmt.Println("Error encoding pos data: ", err)
	}

	envelope.Data = dataBuff

	outBuff, err := msgpack.Marshal(envelope)
	if err != nil {
		fmt.Println("Error encoding envelope: ", err)
	}

	if err := p.conn.WriteMessage(websocket.BinaryMessage, outBuff); err != nil {
		fmt.Println("Error writing msg to player: ", err)
	}
}

func (p *Player) handleMsg(rawData []byte, h *Hub) {
	var inData ClientMessage
	if err := msgpack.Unmarshal(rawData, &inData); err != nil {
		fmt.Println("Error unpacking envelope data: ", err)
	}

	if inData.UpdateType == ClientUpdatePos {
		var posData PlayerData
		if err := msgpack.Unmarshal(inData.Data, &posData); err != nil {
			fmt.Println("Client data could not be asserted as type PlayerData.")
		} else {
			p.updatePos(posData, h)
		}
	} else if inData.UpdateType == ClientRecWordle {
		var wordleData WordleReq
		if err := msgpack.Unmarshal(inData.Data, &wordleData); err != nil {
			fmt.Println("Client data could not be asserted as type WordleData.")
		} else {
			wordOfTheDay := h.resourceManager.GetWordle()
			p.updateWordle(wordleData, wordOfTheDay, &h.resourceManager.GuessableWords)
		}
	}

}

func (p *Player) updatePos(posData PlayerData, h *Hub) {
	posData.ID = p.id
	h.in <- posData
}

func (p *Player) updateWordle(data WordleReq, word string, guessables *map[string]bool) {
	// the colors 4 da letters
	valid, status, colors := getColors(data.Guess, data.GuessCount, word, guessables)

	var response WordleRes
	response.Valid = valid
	response.Status = status
	response.Colors = colors
	if status == WIN || status == LOSE {
		response.Solution = word
	} else {
		response.Solution = ""
	}

	p.send(response, ServerSendWordle)
}


