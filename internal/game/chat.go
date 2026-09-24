package game

import (
	"log/slog"
	"strings"

	"nytrpg/internal/protocol"
)

const maxChatLen = 200

// Says msg as the client's player. Chat is global: everyone online hears it,
// clients only show a bubble over speakers they can see.
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
		e := speaker.ent
		out, err := protocol.Encode(protocol.ServerChat, protocol.ChatMsg{ID: e.ID, Msg: msg, Name: e.Name, Char: e.Char, Class: e.Class})
		if err != nil {
			slog.Error("encoding chat", "err", err)
			return
		}
		for _, p := range w.players {
			w.Stats.BytesOut.Add(int64(len(out)))
			p.client.Send(out)
		}
	})
}
