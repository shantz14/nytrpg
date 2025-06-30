package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type lbRow struct {
	Place int `json:"place"` 
	Uname string `json:"uname"` 
	Guesses int `json:"guesses"` 
	Time float64 `json:"time"` 
}

func handleLeaderboard(h *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed, only GET allowed.", http.StatusMethodNotAllowed)
		return
	}
	date := r.URL.Query().Get("date")
	var res []lbRow
	res = h.db.getLeaderboard(date)

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error encoding leaderboard response:", err)
		return
	}
}

