package main

import (
	"log"
)

func broadcastChat(chat Chat, h *Hub) {
	//chat.Msg = addUsername(chat, h.db)
	for p := range h.players {
		p.send(chat, ServerSendChat)
	}
}

func addUsername(chat Chat, db *Connection) string {
	pRow, found := db.getPlayerById(chat.ID)
	if !found {
		log.Println("Could not find player by ID in addUsername.")
	}
	return pRow.username + ": " + chat.Msg
}
