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

	r.Handle(protocol.ClientDuelChallenge, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.DuelChallengeReq
		if err := msgpack.Unmarshal(data, &req); err != nil || !s.Allow("duelChallenge", 0.5, 3) {
			return
		}
		w.Challenge(s, req.Target, req.Ranked)
	})
	r.Handle(protocol.ClientProfile, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.ProfileReq
		if err := msgpack.Unmarshal(data, &req); err != nil || !s.Allow("profile", 2, 5) {
			return
		}
		if prof, ok := w.Profile(s, req.Target); ok {
			s.SendMsg(protocol.ServerProfile, prof)
		}
	})
	r.Handle(protocol.ClientDuelRespond, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.DuelRespondReq
		if err := msgpack.Unmarshal(data, &req); err != nil || !s.Allow("duelRespond", 2, 5) {
			return
		}
		w.RespondDuel(s, req.ID, req.Accept)
	})
	r.Handle(protocol.ClientDuelGuess, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.WordleReq
		if err := msgpack.Unmarshal(data, &req); err != nil || !s.Allow("duelGuess", 2, 5) {
			return
		}
		w.DuelGuess(s, req.Guess)
	})
	r.Handle(protocol.ClientDuelTyping, func(s *netconn.Session, data msgpack.RawMessage) {
		var req protocol.DuelTyping
		if err := msgpack.Unmarshal(data, &req); err != nil || !s.Allow("duelTyping", 20, 10) {
			return
		}
		w.DuelTyping(s, req.Count)
	})
	r.Handle(protocol.ClientDuelForfeit, func(s *netconn.Session, _ msgpack.RawMessage) {
		w.ForfeitDuel(s)
	})
}
