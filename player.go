package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

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
	Username string `msgpack:"username"`
}

func handleWS(h *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Connection failed at Upgrader: ", err)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr); if err != nil {
		log.Println("ID not a int?", err)
		return
	}

	log.Println("New connection coming from: ", conn.RemoteAddr())

	newPlayer := Player{id: id, conn: conn}

	// Add new conn to set
	h.players[&newPlayer] = true

	pRow, _ := h.db.getPlayerById(id)

	// Add pos to gamestate
	h.state.Players[newPlayer.id] = &PlayerData{ID: newPlayer.id, Pos: Vector2D{X: 0, Y: 0}, Me: false, Username: pRow.username}

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
				log.Println("Player disconnected")
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("Error reading msg from player: ", err)
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
			newPlayer.Username = player.Username

			if (id == p.id) {
				newPlayer.Me = true
			} else {
				newPlayer.Me = false
			}

			playerData.Players[id] = &newPlayer
		}


		playerData.Unregister = h.state.Unregister

		p.send(playerData, ServerUpdatePos)

		if h.state.Unregister != -999 {
			h.state.Unregister = -999
		}

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
	} else if (msgType == ServerSendChat) {
		asserted = data.(Chat)
	}

	dataBuff, err := msgpack.Marshal(asserted)
	if err != nil {
		log.Println("Error encoding pos data: ", err)
	}

	envelope.Data = dataBuff

	outBuff, err := msgpack.Marshal(envelope)
	if err != nil {
		log.Println("Error encoding envelope: ", err)
	}

	if err := p.conn.WriteMessage(websocket.BinaryMessage, outBuff); err != nil {
		log.Println("Error writing msg to player: ", err)
	}
}

func (p *Player) handleMsg(rawData []byte, h *Hub) {
	var inData ClientMessage
	if err := msgpack.Unmarshal(rawData, &inData); err != nil {
		log.Println("Error unpacking envelope data: ", err)
	}

	if inData.UpdateType == ClientUpdatePos {
		var posData PlayerData
		if err := msgpack.Unmarshal(inData.Data, &posData); err != nil {
			log.Println("Client data could not be asserted as type PlayerData.")
		} else {
			p.updatePos(posData, h)
		}
	} else if inData.UpdateType == ClientRecWordle {
		var wordleData WordleReq
		if err := msgpack.Unmarshal(inData.Data, &wordleData); err != nil {
			log.Println("Client data could not be asserted as type WordleReq.")
		} else {
			wordOfTheDay := h.resourceManager.GetWordle()
			p.updateWordle(wordleData, wordOfTheDay, &h.resourceManager.GuessableWords, h.db)
		}
	} else if inData.UpdateType == ClientRecChat {
		var chat Chat
		if err := msgpack.Unmarshal(inData.Data, &chat); err != nil {
			log.Println("Client data could not be asserted as type Chat.")
		} else {
			h.chatIn <- chat
		}
	}

}

func (p *Player) updatePos(posData PlayerData, h *Hub) {
	posData.ID = p.id
	h.in <- posData
}

func (p *Player) updateWordle(data WordleReq, word string, guessables *map[string]bool, db *Connection) {
	// the colors 4 da letters
	valid, status, colors := colorMyBoxes(data.Guess, data.GuessCount, word, guessables)

	var response WordleRes
	response.Valid = valid
	response.Status = status
	response.Colors = colors
	if status == WIN {
		response.Solution = word
		p.submitWordle(true, data, db)
	} else if status == LOSE {
		response.Solution = word
		p.submitWordle(false, data, db)
	} else {
		response.Solution = ""
	}

	p.send(response, ServerSendWordle)
}

func (p *Player) submitWordle(win bool, data WordleReq, db *Connection) {
	now := time.Now().UTC()
	now.Add(time.Duration(-7) * time.Hour)
	date := now.Format("2006-01-02")
	db.insertWordle(date, win, float32(data.Time), data.GuessCount, p.id)
}

