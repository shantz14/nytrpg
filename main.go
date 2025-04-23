package main

import (
	"fmt"
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

    fmt.Println("Server running on port", PORT)
    http.ListenAndServe("localhost:" + strconv.Itoa(PORT), nil)
}
