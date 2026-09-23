// Package protocol defines every message sent over the websocket.
// Keep in sync with client/src/messages.ts.
package protocol

import "github.com/vmihailenco/msgpack/v5"

type ServerMessageType int

const (
	// Data sent FROM the SERVER
	ServerUpdatePos    ServerMessageType = 1
	ServerSendWordle   ServerMessageType = 2
	ServerWordleResume ServerMessageType = 3
	ServerSendChat     ServerMessageType = 4
)

type ServerMessage struct {
	UpdateType ServerMessageType `msgpack:"updateType"`
	Data       []byte            `msgpack:"data"`
}

type ClientMessageType int

const (
	// Data sent FROM the CLIENT
	ClientUpdatePos   ClientMessageType = 1
	ClientRecWordle   ClientMessageType = 2
	ClientRecChat     ClientMessageType = 3
	ClientStartWordle ClientMessageType = 4
)

type ClientMessage struct {
	UpdateType ClientMessageType `msgpack:"updateType"`
	Data       []byte            `msgpack:"data"`
}

// Encodes a server message ready to write to the socket
func Encode(t ServerMessageType, data any) ([]byte, error) {
	dataBuff, err := msgpack.Marshal(data)
	if err != nil {
		return nil, err
	}
	return msgpack.Marshal(ServerMessage{UpdateType: t, Data: dataBuff})
}

type Vector2D struct {
	X float64 `msgpack:"x"`
	Y float64 `msgpack:"y"`
}

type PlayerData struct {
	ID  int      `msgpack:"id"`
	Pos Vector2D `msgpack:"pos"`
	// True when this is the data of the player being sent to
	Me       bool   `msgpack:"me"`
	Username string `msgpack:"username"`
}

// Full snapshot of every connected player. Clients replace their view with each one,
// so anyone missing from a snapshot has left.
type GameState struct {
	Players map[int]*PlayerData `msgpack:"players"`
}

type Chat struct {
	ID  int    `msgpack:"id"`
	Msg string `msgpack:"msg"`
}

type WordleStatus int

const (
	WordleInGame WordleStatus = 0
	WordleWin    WordleStatus = 1
	WordleLose   WordleStatus = 2
)

type WordleColor int

const (
	Grey   WordleColor = 0
	Yellow WordleColor = 1
	Green  WordleColor = 2
)

type WordleReq struct {
	Guess string `msgpack:"guess"`
}

type WordleRes struct {
	Valid    bool          `msgpack:"valid"`
	Status   WordleStatus  `msgpack:"status"`
	Colors   []WordleColor `msgpack:"colors"`
	Solution string        `msgpack:"solution"`
	Seconds  float64       `msgpack:"seconds"`
}

// Sent in reply to ClientStartWordle, the guesses already made today
type WordleResume struct {
	Guesses []string        `msgpack:"guesses"`
	Colors  [][]WordleColor `msgpack:"colors"`
	Seconds float64         `msgpack:"seconds"`
}
