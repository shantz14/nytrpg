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
	ClientMove          ClientMsg = 1 // Vec, the player's new position
	ClientWordleGuess   ClientMsg = 2 // WordleReq
	ClientChat          ClientMsg = 3 // ChatReq
	ClientWordleStart   ClientMsg = 4 // empty, opens today's wordle
	ClientDuelChallenge ClientMsg = 5 // DuelChallengeReq
	ClientDuelRespond   ClientMsg = 6 // DuelRespondReq, accept or deny a challenge
	ClientDuelGuess     ClientMsg = 7 // WordleReq, a guess in your duel
	ClientDuelForfeit   ClientMsg = 8 // empty, give up your duel
	ClientDuelTyping    ClientMsg = 9 // DuelTyping, letters in your current row
)

// Messages sent by the server
type ServerMsg uint8

const (
	ServerWelcome             ServerMsg = 1  // Welcome, first message after connecting
	ServerWordleResult        ServerMsg = 2  // WordleRes
	ServerWordleResume        ServerMsg = 3  // WordleResume
	ServerChat                ServerMsg = 4  // ChatMsg
	ServerWorld               ServerMsg = 5  // WorldUpdate
	ServerCorrection          ServerMsg = 6  // Vec, the server rejected a move, snap back here
	ServerDuelChallenge       ServerMsg = 7  // DuelChallenge, someone challenged you
	ServerDuelChallengeUpdate ServerMsg = 8  // DuelChallengeUpdate, what happened to a challenge
	ServerDuelStart           ServerMsg = 9  // DuelStart, a duel you're in began
	ServerDuelGuess           ServerMsg = 10 // WordleRes, the result of your duel guess
	ServerDuelOpponentGuess   ServerMsg = 11 // DuelOpponentGuess, your opponent guessed
	ServerDuelEnd             ServerMsg = 12 // DuelEnd, your duel is over
	ServerDuelTyping          ServerMsg = 13 // DuelTyping, your opponent's current row
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
	// Sign drawn above it, e.g. "Daily Wordle". Empty = none.
	Label string `json:"label,omitempty" msgpack:"label,omitempty"`
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
	// Who said it, so the chat log can name speakers the client can't see:
	// the account's username, and the character's name and class ID
	Name  string `msgpack:"name"`
	Char  string `msgpack:"char,omitempty"`
	Class string `msgpack:"class,omitempty"`
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

type DuelChallengeReq struct {
	// The player to challenge, must be in view
	Target EntityID `msgpack:"target"`
}

type DuelRespondReq struct {
	// From DuelChallenge
	ID     uint32 `msgpack:"id"`
	Accept bool   `msgpack:"accept"`
}

// Sent to the player being challenged
type DuelChallenge struct {
	ID uint32 `msgpack:"id"`
	// Who is challenging: their entity, username, character name and class ID
	From  EntityID `msgpack:"from"`
	Name  string   `msgpack:"name"`
	Char  string   `msgpack:"char,omitempty"`
	Class string   `msgpack:"class,omitempty"`
	// How long until it expires
	ExpiresMs int `msgpack:"expiresMs"`
}

type DuelChallengeStatus int

const (
	// To the challenger: the challenge is waiting for an answer
	DuelSent DuelChallengeStatus = 0
	// To the challenger: they said no
	DuelDeclined DuelChallengeStatus = 1
	// To both: nobody answered in time
	DuelExpired DuelChallengeStatus = 2
	// To both: someone left, or one of you started another duel
	DuelCancelled DuelChallengeStatus = 3
	// To the challenger: one of you is already in a duel
	DuelBusy DuelChallengeStatus = 4
	// To the challenger: they're gone or not in view
	DuelUnavailable DuelChallengeStatus = 5
)

type DuelChallengeUpdate struct {
	// 0 when the challenge was refused before it got an id (Busy, Unavailable)
	ID uint32 `msgpack:"id"`
	// The other player's character name, or username if they have none
	Name   string              `msgpack:"name"`
	Status DuelChallengeStatus `msgpack:"status"`
}

type DuelStart struct {
	// Who you're up against
	Name       string `msgpack:"name"`
	Char       string `msgpack:"char,omitempty"`
	Class      string `msgpack:"class,omitempty"`
	WordLength int    `msgpack:"wordLength"`
	MaxGuesses int    `msgpack:"maxGuesses"`
}

// The colors of the opponent's guess, never the letters
type DuelOpponentGuess struct {
	Colors []WordleColor `msgpack:"colors"`
}

// How many letters are in the current row, never which ones. Sent by the
// client as it types and forwarded to the opponent.
type DuelTyping struct {
	Count int `msgpack:"count"`
}

type DuelOutcome int

const (
	DuelWin  DuelOutcome = 0
	DuelLose DuelOutcome = 1
	DuelDraw DuelOutcome = 2
)

type DuelEndReason int

const (
	// Someone found the word
	DuelSolved DuelEndReason = 0
	// Both ran out of guesses
	DuelOutOfGuesses DuelEndReason = 1
	// Someone gave up
	DuelForfeit DuelEndReason = 2
	// Someone disconnected
	DuelDisconnect DuelEndReason = 3
)

type DuelEnd struct {
	Outcome  DuelOutcome   `msgpack:"outcome"`
	Reason   DuelEndReason `msgpack:"reason"`
	Solution string        `msgpack:"solution"`
	Seconds  float64       `msgpack:"seconds"`
}
