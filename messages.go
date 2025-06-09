package main

type ServerMessageType int

const (
	ServerUpdatePos ServerMessageType = 1
	ServerSendWordle ServerMessageType = 2
)

type ServerMessage struct {
	UpdateType ServerMessageType `msgpack:"updateType"`
	Data []byte `msgpack:"data"`
}

type ClientMessageType int

const (
	ClientUpdatePos ClientMessageType = 1
	ClientRecWordle ClientMessageType = 2
)

type ClientMessage struct {
	UpdateType ClientMessageType `msgpack:"updateType"`
	Data []byte `msgpack:"data"`
}

type WordleReq struct {
	ID int `msgpack:"id"`
	Guess string `msgpack:"guess"`
	GuessCount int `msgpack:"guessCount"`
	Time int `msgpack:"time"`
}

type WordleRes struct {
	Valid bool `msgpack:"valid"`
	Status WordleStatus `msgpack:"status"`
	Colors []WordleColor `msgpack:"colors"`
	Solution string `msgpack:"solution"`
}


