package main

type ServerMessageType int

const (
	// Data sent FROM the SERVER
	ServerUpdatePos ServerMessageType = 1
	ServerSendWordle ServerMessageType = 2
	ServerWordleResume ServerMessageType = 3
	ServerSendChat ServerMessageType = 4
)

type ServerMessage struct {
	UpdateType ServerMessageType `msgpack:"updateType"`
	Data []byte `msgpack:"data"`
}

type ClientMessageType int

const (
	// Data sent FROM the CLIENT
	ClientUpdatePos ClientMessageType = 1
	ClientRecWordle ClientMessageType = 2
	ClientRecChat ClientMessageType = 3
	ClientStartWordle ClientMessageType = 4
)

type ClientMessage struct {
	UpdateType ClientMessageType `msgpack:"updateType"`
	Data []byte `msgpack:"data"`
}

type WordleReq struct {
	Guess string `msgpack:"guess"`
}

type WordleRes struct {
	Valid bool `msgpack:"valid"`
	Status WordleStatus `msgpack:"status"`
	Colors []WordleColor `msgpack:"colors"`
	Solution string `msgpack:"solution"`
	Seconds float64 `msgpack:"seconds"`
}

type Chat struct {
	ID int `msgpack:"id"`
	Msg string `msgpack:"msg"`
}



// Sent in reply to ClientStartWordle, the guesses already made today
type WordleResume struct {
	Guesses []string `msgpack:"guesses"`
	Colors [][]WordleColor `msgpack:"colors"`
	Seconds float64 `msgpack:"seconds"`
}
