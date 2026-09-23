// Package protocol defines every message sent over the websocket.
//
// This file is the single source of truth: client/src/protocol.gen.ts is generated
// from it. After changing anything here run `go generate ./internal/protocol`.
//
// Every message is a msgpack array [type, payload].
package protocol

import "github.com/vmihailenco/msgpack/v5"

//go:generate go run ../../cmd/protogen -out ../../client/src/protocol.gen.ts

// Messages sent by the client
type ClientMsg uint8

const (
	ClientMove        ClientMsg = 1 // Vec, the player's new position
	ClientWordleGuess ClientMsg = 2 // WordleReq
	ClientChat        ClientMsg = 3 // ChatReq
	ClientWordleStart ClientMsg = 4 // empty, opens today's wordle
)

// Messages sent by the server
type ServerMsg uint8

const (
	ServerWelcome      ServerMsg = 1 // Welcome, first message after connecting
	ServerWordleResult ServerMsg = 2 // WordleRes
	ServerWordleResume ServerMsg = 3 // WordleResume
	ServerChat         ServerMsg = 4 // ChatMsg
	ServerSnapshot     ServerMsg = 5 // Snapshot
)

type envelope struct {
	_msgpack struct{} `msgpack:",as_array"`
	Type     uint8
	Data     msgpack.RawMessage
}

func encode(t uint8, data any) ([]byte, error) {
	raw, err := msgpack.Marshal(data)
	if err != nil {
		return nil, err
	}
	return msgpack.Marshal(envelope{Type: t, Data: raw})
}

// Encodes a server message ready to write to the socket
func Encode(t ServerMsg, data any) ([]byte, error) {
	return encode(uint8(t), data)
}

// Encodes a client message, for bots and tests
func EncodeClient(t ClientMsg, data any) ([]byte, error) {
	return encode(uint8(t), data)
}

// Splits a message into its type and still encoded payload
func Decode(msg []byte) (uint8, msgpack.RawMessage, error) {
	var e envelope
	err := msgpack.Unmarshal(msg, &e)
	return e.Type, e.Data, err
}

// A position in world pixels
type Vec struct {
	X int32 `msgpack:"x"`
	Y int32 `msgpack:"y"`
}

type Welcome struct {
	PlayerID int    `msgpack:"playerId"`
	Username string `msgpack:"username"`
}

type PlayerSnap struct {
	ID       int    `msgpack:"id"`
	Username string `msgpack:"username"`
	Pos      Vec    `msgpack:"pos"`
}

// Every connected player. Clients replace their view with each one,
// so anyone missing from a snapshot has left.
type Snapshot struct {
	Players []PlayerSnap `msgpack:"players"`
}

type ChatReq struct {
	Msg string `msgpack:"msg"`
}

type ChatMsg struct {
	// Player who said it
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

// Sent in reply to ClientWordleStart, the guesses already made today
type WordleResume struct {
	Guesses []string        `msgpack:"guesses"`
	Colors  [][]WordleColor `msgpack:"colors"`
	Seconds float64         `msgpack:"seconds"`
}
