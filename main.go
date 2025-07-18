package main

import (
	"log"
	"net/http"
	"strconv"

)

const PORT = 8080

var addr = ""

func main() {

    static := http.Dir("client/static")

	hub := newHub()
	go hub.run()

    http.Handle("/", http.FileServer(static))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(hub, w, r)
	})
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		handleLogin(hub, w, r)
	})
	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		checkToken(hub, w, r)
	})
	http.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		handleSignup(hub, w, r)
	})
	http.HandleFunc("/haveIPlayed", func(w http.ResponseWriter, r *http.Request) {
		handleHaveIPlayed(hub, w, r)
	})
	http.HandleFunc("/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		handleLeaderboard(hub, w, r)
	})

    log.Println("Server running on port", PORT)
    http.ListenAndServe(":" + strconv.Itoa(PORT), nil)
}
