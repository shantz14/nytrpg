package game

import (
	"github.com/vmihailenco/msgpack/v5"

	"nytrpg/internal/netconn"
	"nytrpg/internal/protocol"
)

// Registers the websocket messages the world handles
func (w *World) RegisterHandlers(r *netconn.Router) {
	r.Handle(protocol.ClientMove, func(s *netconn.Session, data msgpack.RawMessage) {
		var pos protocol.Vec
		if err := msgpack.Unmarshal(data, &pos); err != nil {
			return
		}
		w.Move(s, pos)
	})
	r.Handle(protocol.ClientChat, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.ChatReq
		if err := msgpack.Unmarshal(data, &req); err != nil {
			return
		}
		if !s.Allow("chat", 1, 3) {
			return
		}
		w.Chat(s, req.Msg)
	})
}
