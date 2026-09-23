package main

import (
	"log"
	"net/http"
	"strconv"

	"nytrpg/internal/config"
	"nytrpg/internal/server"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer srv.Close()

	log.Println("Server running on port", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.Port), srv.Handler()))
}
