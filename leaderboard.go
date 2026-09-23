package main

import (
	"encoding/json"
	"log"
	"net/http"
	"nytrpg/resources"
	"strconv"
	"time"
)

const LB_PAGE_SIZE = 10

type lbRow struct {
	Place int `json:"place"`
	Uname string `json:"uname"`
	Guesses int `json:"guesses"`
	Time float64 `json:"time"`
}

type lbRes struct {
	Date string `json:"date"`
	Today string `json:"today"`
	Page int `json:"page"`
	PageSize int `json:"pageSize"`
	Total int `json:"total"`
	Rows []lbRow `json:"rows"`
}

func handleLeaderboard(h *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed, only GET allowed.", http.StatusMethodNotAllowed)
		return
	}

	var res lbRes
	res.Today = resources.Today()
	res.PageSize = LB_PAGE_SIZE

	res.Date = r.URL.Query().Get("date")
	if res.Date == "" {
		res.Date = res.Today
	} else if _, err := time.Parse(resources.DateLayout, res.Date); err != nil {
		http.Error(w, "Bad date, use YYYY-MM-DD.", http.StatusBadRequest)
		return
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 0 {
			http.Error(w, "Bad page.", http.StatusBadRequest)
			return
		}
		res.Page = page
	}

	rows, total, err := h.db.getLeaderboard(res.Date, LB_PAGE_SIZE, res.Page*LB_PAGE_SIZE)
	if err != nil {
		log.Println("Error getting leaderboard:", err)
		http.Error(w, "Database error.", http.StatusInternalServerError)
		return
	}
	res.Rows = rows
	res.Total = total

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		log.Println("Error encoding leaderboard response:", err)
		return
	}
}
