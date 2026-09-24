// Package protocol defines every message sent over the websocket.
//
// This file is the single source of truth: client/src/protocol.gen.ts is generated
// from it. After changing anything here run `go generate ./internal/protocol`.
//
// Every message is a msgpack array [type, payload].
package protocol

import (
	"bytes"

	"github.com/vmihailenco/msgpack/v5"
)

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
	ServerWorld        ServerMsg = 5 // WorldUpdate
	ServerCorrection   ServerMsg = 6 // Vec, the server rejected a move, snap back here
)

type envelope struct {
	_msgpack struct{} `msgpack:",as_array"`
	Type     uint8
	Data     msgpack.RawMessage
}

func encode(t uint8, data any) ([]byte, error) {
	raw, err := marshal(data)
	if err != nil {
		return nil, err
	}
	return marshal(envelope{Type: t, Data: raw})
}

// Like msgpack.Marshal, but writes ints in as few bytes as they need. By default
// int32/uint32 always take 5 bytes, which made an entity move 16 bytes, not 4.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := msgpack.GetEncoder()
	defer msgpack.PutEncoder(enc)
	enc.Reset(&buf)
	enc.UseCompactInts(true)
	err := enc.Encode(v)
	return buf.Bytes(), err
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

type EntityID uint32

type EntityKind uint8

const (
	EntityPlayer EntityKind = 1
)

type Welcome struct {
	PlayerID int    `msgpack:"playerId"`
	Username string `msgpack:"username"`
	// The entity that is you. It is never sent in WorldUpdates, you move it yourself.
	EntityID EntityID `msgpack:"entityId"`
	Pos      Vec      `msgpack:"pos"`
	Map      WorldMap `msgpack:"map"`
	// Fastest a player may move in px/s, faster moves are corrected
	MoveSpeed float64 `msgpack:"moveSpeed"`
	TickRate  int     `msgpack:"tickRate"`
	// The character you're playing
	Character CharacterInfo `msgpack:"character"`
	// Every class, to look up the class of other players
	Classes []ClassInfo `msgpack:"classes"`
}

// One of an account's characters. Also sent as JSON by /characters.
type CharacterInfo struct {
	ID   int    `json:"id" msgpack:"id"`
	Slot int    `json:"slot" msgpack:"slot"`
	Name string `json:"name" msgpack:"name"`
	// Class ID, see ClassInfo
	Class string `json:"class" msgpack:"class"`
}

type ClassInfo struct {
	ID          string `json:"id" msgpack:"id"`
	Name        string `json:"name" msgpack:"name"`
	Description string `json:"description" msgpack:"description"`
	// Always one per ability slot, an empty ID means the slot is empty
	Abilities []AbilityInfo `json:"abilities" msgpack:"abilities"`
}

type AbilityInfo struct {
	ID          string `json:"id" msgpack:"id"`
	Name        string `json:"name" msgpack:"name"`
	Description string `json:"description" msgpack:"description"`
}

// The static world, loaded from a JSON map file
type WorldMap struct {
	Width         int32          `json:"width" msgpack:"width"`
	Height        int32          `json:"height" msgpack:"height"`
	Background    string         `json:"background" msgpack:"background"`
	Spawn         Vec            `json:"spawn" msgpack:"spawn"`
	Interactables []Interactable `json:"interactables" msgpack:"interactables"`
}

// Something in the world a player can click
type Interactable struct {
	ID     string `json:"id" msgpack:"id"`
	Sprite string `json:"sprite" msgpack:"sprite"`
	Pos    Vec    `json:"pos" msgpack:"pos"`
	W      int32  `json:"w" msgpack:"w"`
	H      int32  `json:"h" msgpack:"h"`
	// What the client does when it's clicked, e.g. "wordle"
	Action string `json:"action" msgpack:"action"`
	// How close a player must be to use it, px from its edge. 0 = anywhere.
	Range int32 `json:"range" msgpack:"range"`
}

// Changes to the entities near you since the last update. Only sent when
// something changed.
type WorldUpdate struct {
	// Entities that came into view
	Spawn []EntitySpawn `msgpack:"spawn,omitempty"`
	// Known entities that moved
	Move []EntityMove `msgpack:"move,omitempty"`
	// Entities that left view or the game
	Despawn []EntityID `msgpack:"despawn,omitempty"`
}

type EntitySpawn struct {
	ID   EntityID   `msgpack:"id"`
	Kind EntityKind `msgpack:"kind"`
	// For players, the account's username
	Name string `msgpack:"name"`
	// For players, the character's name and class ID
	Char   string `msgpack:"char,omitempty"`
	Class  string `msgpack:"class,omitempty"`
	Sprite string `msgpack:"sprite"`
	Pos    Vec    `msgpack:"pos"`
}

// Sent as [id, x, y] to keep moves small
type EntityMove struct {
	_msgpack struct{} `msgpack:",as_array"`
	ID       EntityID
	X        int32
	Y        int32
}

type ChatReq struct {
	Msg string `msgpack:"msg"`
}

type ChatMsg struct {
	// Entity who said it
	ID  EntityID `msgpack:"id"`
	Msg string   `msgpack:"msg"`
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
	// Already finished today's wordle, nothing else is set
	Played bool `msgpack:"played"`
	// Not close enough to the wordle board to start, nothing else is set
	TooFar  bool            `msgpack:"tooFar"`
	Guesses []string        `msgpack:"guesses"`
	Colors  [][]WordleColor `msgpack:"colors"`
	Seconds float64         `msgpack:"seconds"`
}
