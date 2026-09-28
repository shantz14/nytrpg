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
	ClientMove          ClientMsg = 1  // Vec, the player's new position
	ClientWordleGuess   ClientMsg = 2  // WordleReq
	ClientChat          ClientMsg = 3  // ChatReq
	ClientWordleStart   ClientMsg = 4  // empty, opens today's wordle
	ClientDuelChallenge ClientMsg = 5  // DuelChallengeReq
	ClientDuelRespond   ClientMsg = 6  // DuelRespondReq, accept or deny a challenge
	ClientDuelGuess     ClientMsg = 7  // WordleReq, a guess in your duel
	ClientDuelForfeit   ClientMsg = 8  // empty, give up your duel
	ClientDuelTyping    ClientMsg = 9  // DuelTyping, letters in your current row
	ClientProfile       ClientMsg = 10 // ProfileReq, a player's ranked profile
	ClientDuelCast      ClientMsg = 11 // DuelCastReq, use an ability in your duel
	ClientIllusionGuess ClientMsg = 12 // WordleReq, a guess in the illusion you're trapped in
	ClientDuelReshape   ClientMsg = 13 // DuelReshapeReq, the word picked for Reshape Reality
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
	ServerProfile             ServerMsg = 14 // Profile, in reply to ClientProfile
	ServerDuelState           ServerMsg = 15 // DuelState, energy and effects on both sides, whenever they change
	ServerDuelCast            ServerMsg = 16 // DuelCast, an ability was used or landed in your duel
	ServerDuelBoard           ServerMsg = 17 // DuelBoard, a board's colors changed (Reshape Reality)
	ServerDuelScry            ServerMsg = 18 // DuelScry, the answer to your Scry
	ServerDuelEyes            ServerMsg = 19 // DuelEyes, the opponent's letters your Seeing Eyes show
	ServerDuelReshapeOptions  ServerMsg = 20 // DuelReshapeOptions, words to pick for Reshape Reality
	ServerIllusionStart       ServerMsg = 21 // IllusionStart, you're trapped in an illusion
	ServerIllusionGuess       ServerMsg = 22 // WordleRes, the result of your illusion guess
	ServerIllusionEnd         ServerMsg = 23 // IllusionEnd, you escaped the illusion or failed it
	ServerDuelPickpocket      ServerMsg = 24 // DuelPickpocket, the letter your Pickpocket revealed
	ServerDuelPrayer          ServerMsg = 25 // DuelPrayer, your prayer was heard, answered or went unanswered
	ServerDuelDivine          ServerMsg = 26 // DuelDivine, Divine Intervention changed the duel
	ServerDuelGuessRemoved    ServerMsg = 27 // DuelGuessRemoved, a guess was mended away
	ServerDuelReveal          ServerMsg = 28 // DuelReveal, the gods revealed a letter of your word
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
	// Your character's ranked rating
	Elo int `msgpack:"elo"`
	// Every rank, lowest first, to name and color anyone's elo
	Ladder []RankTier `msgpack:"ladder"`
}

// One rank on the ranked ladder
type RankTier struct {
	// e.g. "silver-2"
	ID string `msgpack:"id"`
	// e.g. "silver", for its color
	Family string `msgpack:"family"`
	// e.g. "Silver 2"
	Name string `msgpack:"name"`
	// Lowest elo in this rank
	MinElo int `msgpack:"minElo"`
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
	// What players of this class look like, an image in client/static/assets
	Sprite string `json:"sprite" msgpack:"sprite"`
	// Always one per ability slot, an empty ID means the slot is empty
	Abilities []AbilityInfo `json:"abilities" msgpack:"abilities"`
	Passive   PassiveInfo   `json:"passive" msgpack:"passive"`
}

type AbilityInfo struct {
	ID          string `json:"id" msgpack:"id"`
	Name        string `json:"name" msgpack:"name"`
	Description string `json:"description" msgpack:"description"`
	// Energy it takes to cast
	Cost int `json:"cost" msgpack:"cost"`
	// Can only be cast once a duel
	Once bool `json:"once" msgpack:"once"`
	// What the caster picks when casting it
	Target AbilityTarget `json:"target" msgpack:"target"`
	// Image in client/static/assets/abilities
	Icon string `json:"icon" msgpack:"icon"`
}

type AbilityTarget int

const (
	// Nothing to pick, it's cast right away
	TargetNone AbilityTarget = 0
	// A letter tile in one of the opponent's guesses
	TargetOpponentTile AbilityTarget = 1
	// A letter A-Z
	TargetLetter AbilityTarget = 2
	// A word from options the server sends (DuelReshapeOptions)
	TargetWord AbilityTarget = 3
	// One of your own guesses, by row
	TargetOwnRow AbilityTarget = 4
	// A color for each letter, not all green
	TargetColors AbilityTarget = 5
)

// Always on, set off by something in the duel. An empty ID means none.
type PassiveInfo struct {
	ID          string `json:"id" msgpack:"id"`
	Name        string `json:"name" msgpack:"name"`
	Description string `json:"description" msgpack:"description"`
	Icon        string `json:"icon" msgpack:"icon"`
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
	// Known players whose ranked rating changed
	Elo []EntityElo `msgpack:"elo,omitempty"`
}

// Sent as [id, elo]
type EntityElo struct {
	_msgpack struct{} `msgpack:",as_array"`
	ID       EntityID
	Elo      int
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
	// For players, their character's ranked rating
	Elo int `msgpack:"elo"`
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
	// Duels only: a letter destroyed by Slash, its color is gone
	Hidden WordleColor = 3
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
	// Duels only: the guess wasn't taken because you're stunned or trapped in
	// an illusion. Valid is false.
	Blocked bool `msgpack:"blocked"`
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
	// Ranked duels change both players' elo
	Ranked bool `msgpack:"ranked"`
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
	ExpiresMs int  `msgpack:"expiresMs"`
	Ranked    bool `msgpack:"ranked"`
	// The challenger's elo
	Elo int `msgpack:"elo"`
	// Ranked only: what you'd win or lose against them
	Stakes *Stakes `msgpack:"stakes,omitempty"`
}

// The elo you'd gain by winning and lose by losing a ranked duel against
// someone, smallest to largest depending on the margin. Losses are negative.
type Stakes struct {
	WinMin  int `msgpack:"winMin"`
	WinMax  int `msgpack:"winMax"`
	LoseMin int `msgpack:"loseMin"`
	LoseMax int `msgpack:"loseMax"`
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
	Ranked     bool   `msgpack:"ranked"`
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
	// Divine Intervention's time limit ran out, a draw
	DuelTimeUp DuelEndReason = 4
)

type DuelEnd struct {
	Outcome  DuelOutcome   `msgpack:"outcome"`
	Reason   DuelEndReason `msgpack:"reason"`
	Solution string        `msgpack:"solution"`
	Seconds  float64       `msgpack:"seconds"`
	// Ranked duels only: your elo before and after, how likely you were to win
	// going in (0-1), and the margin multiplier (1 to 1.75) applied
	Ranked    bool    `msgpack:"ranked"`
	EloBefore int     `msgpack:"eloBefore"`
	EloAfter  int     `msgpack:"eloAfter"`
	Expected  float64 `msgpack:"expected"`
	Margin    float64 `msgpack:"margin"`
}

type ProfileReq struct {
	// A player in view, or yourself
	Target EntityID `msgpack:"target"`
}

// A player's ranked record
type Profile struct {
	ID    EntityID `msgpack:"id"`
	Name  string   `msgpack:"name"`
	Char  string   `msgpack:"char"`
	Class string   `msgpack:"class"`
	Elo   int      `msgpack:"elo"`
	// Highest elo ever reached
	Peak   int `msgpack:"peak"`
	Games  int `msgpack:"games"`
	Wins   int `msgpack:"wins"`
	Losses int `msgpack:"losses"`
	Draws  int `msgpack:"draws"`
	// Latest first
	Recent []RankedMatchInfo `msgpack:"recent"`
	// What you'd win or lose against them, unset for your own profile
	Stakes *Stakes `msgpack:"stakes,omitempty"`
}

// One ranked duel, from the profile owner's side
type RankedMatchInfo struct {
	Opponent      string      `msgpack:"opponent"`
	OpponentClass string      `msgpack:"opponentClass"`
	Outcome       DuelOutcome `msgpack:"outcome"`
	// Elo gained, negative for a loss
	Change int `msgpack:"change"`
	// Unix seconds
	PlayedAt int64 `msgpack:"playedAt"`
}

type DuelCastReq struct {
	// The ability slot, 0 to 4
	Slot int `msgpack:"slot"`
	// TargetOpponentTile: the tile in the opponent's guesses
	Row int `msgpack:"row"`
	Col int `msgpack:"col"`
	// TargetLetter: the letter
	Letter string `msgpack:"letter"`
	// TargetOwnRow: the row is Row
	// TargetColors: one per letter
	Colors []WordleColor `msgpack:"colors"`
}

type DuelReshapeReq struct {
	// One of the words from DuelReshapeOptions
	Word string `msgpack:"word"`
}

// Energy and effects on both sides of your duel
type DuelState struct {
	You  DuelSideState `msgpack:"you"`
	Them DuelSideState `msgpack:"them"`
	// Divine Intervention's time limit: the duel is a draw in this long. 0
	// when there's none.
	DeadlineMs int `msgpack:"deadlineMs"`
}

type DuelSideState struct {
	Energy int `msgpack:"energy"`
	// Guess rows they have, used or not
	Rows    int `msgpack:"rows"`
	Guesses int `msgpack:"guesses"`
	// How much longer their keyboard is stunned, 0 when it isn't
	StunnedMs int  `msgpack:"stunnedMs"`
	Shield    bool `msgpack:"shield"`
	// Seeing Eyes they have on their opponent
	Eyes int `msgpack:"eyes"`
	// Magic Missiles flying at them, how long until each lands
	MissilesMs []int `msgpack:"missilesMs"`
	// Trapped in an illusion
	Illusion bool `msgpack:"illusion"`
	// Once-only abilities they already used
	Used []string `msgpack:"used"`
	// How much longer they can't cast abilities, 0 when they can
	SilencedMs int `msgpack:"silencedMs"`
	// Guesses left on a scrambled keyboard, 0 when it isn't
	ScrambledGuesses int `msgpack:"scrambledGuesses"`
	// Your side only: what each key A-Z types while scrambled, "" when it
	// isn't. Key i types Keymap[i].
	Keymap string `msgpack:"keymap"`
	// Your side only: your next guess needn't be a word (Cheat), and shows
	// your opponent colors you picked (Feint)
	CheatReady bool `msgpack:"cheatReady"`
	FeintReady bool `msgpack:"feintReady"`
}

type DuelCastKind int

const (
	// Someone cast it
	CastUsed DuelCastKind = 0
	// A delayed ability hit: a Magic Missile landed, an Illusion was failed
	CastLanded DuelCastKind = 1
	// A delayed ability came to nothing: a Magic Missile beaten by a guess,
	// an Illusion solved
	CastFizzled DuelCastKind = 2
	// A passive went off
	CastTriggered DuelCastKind = 3
)

// Something happened with an ability in your duel, for animations and the
// event feed
type DuelCast struct {
	// You cast it, or it's your passive
	ByYou   bool         `msgpack:"byYou"`
	Ability string       `msgpack:"ability"`
	Kind    DuelCastKind `msgpack:"kind"`
	// A shield stopped it
	Blocked bool `msgpack:"blocked"`
	// Slash: the tile destroyed
	Row int `msgpack:"row"`
	Col int `msgpack:"col"`
}

// Every guess's colors on one board, after they changed
type DuelBoard struct {
	// Your board, or the opponent's
	Yours  bool            `msgpack:"yours"`
	Colors [][]WordleColor `msgpack:"colors"`
}

type DuelScry struct {
	Letter string `msgpack:"letter"`
	InWord bool   `msgpack:"inWord"`
}

// Letters in the opponent's guesses your Seeing Eyes show
type DuelEyes struct {
	Tiles []EyeTile `msgpack:"tiles"`
}

type EyeTile struct {
	Row    int    `msgpack:"row"`
	Col    int    `msgpack:"col"`
	Letter string `msgpack:"letter"`
}

type DuelReshapeOptions struct {
	Words []string `msgpack:"words"`
}

// You must solve this small Wordle before you can guess in your duel again
type IllusionStart struct {
	WordLength int `msgpack:"wordLength"`
	MaxGuesses int `msgpack:"maxGuesses"`
}

type IllusionEnd struct {
	Won      bool   `msgpack:"won"`
	Solution string `msgpack:"solution"`
	// Energy lost for failing it
	EnergyLost int `msgpack:"energyLost"`
	// Purify ended it, neither won nor lost
	Purified bool `msgpack:"purified"`
}

// A letter in the opponent's guesses your Pickpocket revealed
type DuelPickpocket struct {
	Row    int    `msgpack:"row"`
	Col    int    `msgpack:"col"`
	Letter string `msgpack:"letter"`
}

type PrayerKind int

const (
	// A riddle about one letter of your word and where it goes
	PrayerMinor PrayerKind = 0
	// A riddle about your whole word
	PrayerMajor PrayerKind = 1
)

// A Cleric's prayer, sent when it's heard (Pending) and again when a god
// answers or none does (Failed, the energy is given back)
type DuelPrayer struct {
	ID      int        `msgpack:"id"`
	Kind    PrayerKind `msgpack:"kind"`
	Pending bool       `msgpack:"pending"`
	Failed  bool       `msgpack:"failed"`
	// The god who answered: name, title, and a CSS color for them
	God      string `msgpack:"god"`
	GodTitle string `msgpack:"godTitle"`
	GodColor string `msgpack:"godColor"`
	Text     string `msgpack:"text"`
	// Energy given back when no god answered
	Refunded int `msgpack:"refunded"`
}

type DivineFate int

const (
	// Both sides' guesses, energy and effects are wiped, the word stays
	FateCleanSlate DivineFate = 0
	// Both sides get one new word, guesses are scored against it
	FateNewWord DivineFate = 1
	// The duel is a draw if nobody solves in 2 minutes
	FateSuddenDeath DivineFate = 2
	// Both are shown the same position of their word (DuelReveal)
	FateRevelation DivineFate = 3
	// The two sides swap energy
	FateFortune DivineFate = 4
)

// Divine Intervention: what the gods did, and what to proclaim about it
type DuelDivine struct {
	Fate   DivineFate `msgpack:"fate"`
	Banner string     `msgpack:"banner"`
}

// Mend took a guess back: later rows move up one
type DuelGuessRemoved struct {
	Yours bool `msgpack:"yours"`
	Row   int  `msgpack:"row"`
}

// A letter of your word and where it goes, from the gods
type DuelReveal struct {
	Col    int    `msgpack:"col"`
	Letter string `msgpack:"letter"`
}
