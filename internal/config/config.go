// Package config reads server settings from the environment.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port      int
	DBPath    string
	StaticDir string
	JWTSecret []byte
	// Enables /debug/pprof
	Debug bool
	// Every duel is on this word, for end-to-end tests. Empty = a random word.
	DuelWord string
}

func FromEnv() (Config, error) {
	c := Config{
		Port:      8080,
		DBPath:    getenv("DB_PATH", "db/nytrpg.db"),
		StaticDir: getenv("STATIC_DIR", "client/static"),
		JWTSecret: []byte(os.Getenv("JWT_SECRET")),
		Debug:     os.Getenv("DEBUG") == "1",
		DuelWord:  strings.ToUpper(os.Getenv("DUEL_WORD")),
	}
	if p := os.Getenv("PORT"); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return c, errors.New("PORT must be a number")
		}
		c.Port = port
	}
	if len(c.JWTSecret) == 0 {
		return c, errors.New("JWT_SECRET must be set")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
