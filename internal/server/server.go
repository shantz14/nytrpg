// Package server wires every feature together behind one http.Handler.
package server

import (
	"log"
	"net/http"

	"nytrpg/internal/auth"
	"nytrpg/internal/config"
	"nytrpg/internal/game"
	"nytrpg/internal/netconn"
	"nytrpg/internal/puzzles/wordle"
	"nytrpg/internal/store"
)

type Server struct {
	cfg    config.Config
	store  *store.Store
	hub    *game.Hub
	auth   *auth.Service
	wordle *wordle.Service
	router *netconn.Router
}

func New(cfg config.Config) (*Server, error) {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	hub := game.NewHub()
	s := &Server{
		cfg:    cfg,
		store:  st,
		hub:    hub,
		auth:   auth.New(st, cfg.JWTSecret, hub),
		wordle: wordle.NewService(st),
		router: netconn.NewRouter(),
	}

	// Each feature registers the websocket messages it handles
	s.hub.RegisterHandlers(s.router)
	s.wordle.RegisterHandlers(s.router)

	go hub.Run()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(s.cfg.StaticDir)))
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/login", s.auth.HandleLogin)
	mux.HandleFunc("/signup", s.auth.HandleSignup)
	mux.HandleFunc("/token", s.auth.HandleToken)
	mux.HandleFunc("/haveIPlayed", s.wordle.HandleHaveIPlayed)
	mux.HandleFunc("/leaderboard", s.wordle.HandleLeaderboard)
	return mux
}

func (s *Server) Close() error {
	return s.store.Close()
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Authenticate before upgrading, the id comes from the token not the client
	p, ok := s.auth.PlayerFromToken(r.URL.Query().Get("token"))
	if !ok {
		http.Error(w, "Invalid token.", http.StatusUnauthorized)
		return
	}
	if !s.hub.ClaimOnline(p.ID) {
		http.Error(w, "Already connected.", http.StatusConflict)
		return
	}

	// The hub releases the claim when the player leaves
	err := netconn.Serve(w, r, p.ID, p.Username, s.router, s.hub.Join, s.hub.Leave)
	if err != nil {
		log.Println("Connection failed at Upgrader: ", err)
		s.hub.ReleaseOnline(p.ID)
	}
}
