package game

import (
	"log/slog"
	"strings"

	"nytrpg/internal/protocol"
)

const maxChatLen = 200

// Says msg as the client's player. Heard by everyone who can see them.
func (w *World) Chat(c Client, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	if runes := []rune(msg); len(runes) > maxChatLen {
		msg = string(runes[:maxChatLen])
	}

	w.Do(func(w *World) {
		speaker, ok := w.players[c]
		if !ok {
			return
		}
		out, err := protocol.Encode(protocol.ServerChat, protocol.ChatMsg{ID: speaker.ent.ID, Msg: msg})
		if err != nil {
			slog.Error("encoding chat", "err", err)
			return
		}
		for _, p := range w.players {
			if _, sees := p.known[speaker.ent.ID]; sees || p == speaker {
				w.Stats.BytesOut.Add(int64(len(out)))
				p.client.Send(out)
			}
		}
	})
}
