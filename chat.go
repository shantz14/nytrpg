package main

import (
	"strings"
)

const MAX_CHAT_LEN = 200

// Runs on the hub goroutine, so it must never block
func broadcastChat(chat Chat, h *Hub) {
	chat.Msg = strings.TrimSpace(chat.Msg)
	if chat.Msg == "" {
		return
	}
	if runes := []rune(chat.Msg); len(runes) > MAX_CHAT_LEN {
		chat.Msg = string(runes[:MAX_CHAT_LEN])
	}

	for p := range h.players {
		select {
		case p.chatOut <- chat:
		default:
			// Player is backed up, drop it rather than stall everyone
		}
	}
}
