package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type PlayerConnAndState struct {
	playerConn *Player
	playerState *PlayerData
}

type Player struct {
	id int
	conn *websocket.Conn
	stateOut chan GameState
	chatOut chan Chat
	wordleOut chan outMsg
	// Closed when the reader exits, tells the writer to stop
	done chan struct{}
	// Closed when the writer exits, so nothing blocks queueing for it
	writerDone chan struct{}
}

// A message waiting for the writer goroutine
type outMsg struct {
	msgType ServerMessageType
	data any
}

type PlayerData struct {
	ID int `msgpack:"id"`
	Pos Vector2D `msgpack:"pos"`
	// True when this is the data of the player being sent to
	Me bool `msgpack:"me"`
	Username string `msgpack:"username"`
}

func handleWS(h *Hub, w http.ResponseWriter, r *http.Request) {
	// Authenticate before upgrading, the id comes from the token not the client
	verified, uname := verifyToken(r.URL.Query().Get("token"))
	if !verified {
		http.Error(w, "Invalid token.", http.StatusUnauthorized)
		return
	}
	pRow, found := h.db.getPlayerByUname(uname)
	if !found {
		http.Error(w, "Unknown player.", http.StatusUnauthorized)
		return
	}
	if !h.claimOnline(pRow.id) {
		http.Error(w, "Already connected.", http.StatusConflict)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Connection failed at Upgrader: ", err)
		h.releaseOnline(pRow.id)
		return
	}

	log.Println("New connection coming from: ", conn.RemoteAddr())

	newPlayer := Player{
		id: pRow.id,
		conn: conn,
		chatOut: make(chan Chat, 30),
		stateOut: make(chan GameState, 1),
		wordleOut: make(chan outMsg, 4),
		done: make(chan struct{}),
		writerDone: make(chan struct{}),
	}

	playerState := PlayerData{ID: newPlayer.id, Pos: Vector2D{X: 0, Y: 0}, Me: false, Username: pRow.username}

	h.register <- PlayerConnAndState {
		&newPlayer,
		&playerState,
	}

	go newPlayer.writeLoop()
	go newPlayer.readLoop(h)
}

// Handles everything the client sends. Exiting this is what disconnects a player.
func (p *Player) readLoop(h *Hub) {
	defer func() {
		close(p.done)
		p.conn.Close()
		h.unregister <- p
	}()

	for {
		_, inBuff, err := p.conn.ReadMessage() // No use for msg type yet
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Println("Player disconnected")
			} else {
				log.Println("Error reading msg from player: ", err)
			}
			return
		}
		p.handleMsg(inBuff, h)
	}
}

// The only goroutine that writes to the connection
func (p *Player) writeLoop() {
	defer func() {
		close(p.writerDone)
		p.conn.Close()
	}()

	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()

	var lastState GameState
	haveState := false

	for {
		var err error
		select {
		case <-p.done:
			return

		case state := <-p.stateOut:
			lastState = state
			haveState = true
			continue

		case msg := <-p.wordleOut:
			err = p.send(msg.data, msg.msgType)

		case chat := <-p.chatOut:
			err = p.send(chat, ServerSendChat)

		case <-ticker.C:
			if !haveState {
				continue
			}
			err = p.send(p.personalize(lastState), ServerUpdatePos)
			haveState = false
		}

		if err != nil {
			log.Println("Error writing msg to player: ", err)
			return
		}
	}
}

// Copies the shared state, marking this player's own entry
func (p *Player) personalize(state GameState) GameState {
	var playerData GameState
	playerData.Players = make(map[int]*PlayerData, len(state.Players))

	for id, player := range state.Players {
		newPlayer := *player
		newPlayer.ID = id
		newPlayer.Me = id == p.id
		playerData.Players[id] = &newPlayer
	}

	return playerData
}

func (p *Player) send(data any, msgType ServerMessageType) error {
	var envelope ServerMessage
	envelope.UpdateType = msgType

	dataBuff, err := msgpack.Marshal(data)
	if err != nil {
		log.Println("Error encoding data: ", err)
		return nil
	}

	envelope.Data = dataBuff

	outBuff, err := msgpack.Marshal(envelope)
	if err != nil {
		log.Println("Error encoding envelope: ", err)
		return nil
	}

	return p.conn.WriteMessage(websocket.BinaryMessage, outBuff)
}

func (p *Player) handleMsg(rawData []byte, h *Hub) {
	var inData ClientMessage
	if err := msgpack.Unmarshal(rawData, &inData); err != nil {
		log.Println("Error unpacking envelope data: ", err)
		return
	}

	if inData.UpdateType == ClientUpdatePos {
		var posData PlayerData
		if err := msgpack.Unmarshal(inData.Data, &posData); err != nil {
			log.Println("Client data could not be asserted as type PlayerData.")
		} else {
			p.updatePos(posData, h)
		}
	} else if inData.UpdateType == ClientStartWordle {
		resume := h.wordles.start(p.id, time.Now())
		p.queueWordle(outMsg{ServerWordleResume, resume})
	} else if inData.UpdateType == ClientRecWordle {
		var wordleData WordleReq
		if err := msgpack.Unmarshal(inData.Data, &wordleData); err != nil {
			log.Println("Client data could not be asserted as type WordleReq.")
		} else {
			p.updateWordle(wordleData, h)
		}
	} else if inData.UpdateType == ClientRecChat {
		var chat Chat
		if err := msgpack.Unmarshal(inData.Data, &chat); err != nil {
			log.Println("Client data could not be asserted as type Chat.")
		} else {
			// Never trust who the client says sent it
			chat.ID = p.id
			h.chatIn <- chat
		}
	}

}

func (p *Player) updatePos(posData PlayerData, h *Hub) {
	posData.ID = p.id
	h.in <- posData
}

func (p *Player) updateWordle(data WordleReq, h *Hub) {
	played := func(date string) bool {
		played, err := h.db.playedOn(p.id, date)
		// If the db is broken don't let them play
		return played || err != nil
	}
	response, finished := h.wordles.guess(p.id, data.Guess, time.Now(), h.resourceManager, played)
	if finished != nil {
		win := response.Status == WIN
		h.db.insertWordle(finished.date, win, float32(response.Seconds), len(finished.guesses), p.id)
	}

	p.queueWordle(outMsg{ServerSendWordle, response})
}

func (p *Player) queueWordle(msg outMsg) {
	select {
	case p.wordleOut <- msg:
	case <-p.writerDone:
	}
}
