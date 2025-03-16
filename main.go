package main

import (
	"fmt"
	"net/http"
	"strconv"
)

const PORT = 8000

func main() {

    static := http.Dir("client/static")

    http.Handle("/", http.FileServer(static))

    fmt.Println("Server running on port", PORT)
    http.ListenAndServe("localhost:" + strconv.Itoa(PORT), nil)
}
