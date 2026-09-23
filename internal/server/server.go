// Package server wires every feature together behind one http.Handler.
package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

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
	world  *game.World
	auth   *auth.Service
	wordle *wordle.Service
	router *netconn.Router

	stopWorld context.CancelFunc

	// Open websocket sessions, so shutdown can close them and wait
	sessionsMu sync.Mutex
	sessions   map[*netconn.Session]bool
	conns      sync.WaitGroup
}

func New(cfg config.Config) (*Server, error) {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	m, err := game.LoadMap("town")
	if err != nil {
		st.Close()
		return nil, err
	}

	world := game.NewWorld(m)
	s := &Server{
		cfg:      cfg,
		store:    st,
		world:    world,
		auth:     auth.New(st, cfg.JWTSecret),
		wordle:   wordle.NewService(st),
		router:   netconn.NewRouter(),
		sessions: make(map[*netconn.Session]bool),
	}

	// Each feature registers the websocket messages it handles
	s.world.RegisterHandlers(s.router)
	s.wordle.RegisterHandlers(s.router)

	ctx, cancel := context.WithCancel(context.Background())
	s.stopWorld = cancel
	go world.Run(ctx)
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

// Disconnects every player, stops the world, and closes the database. Call after
// the HTTP server has stopped accepting connections.
func (s *Server) Shutdown(ctx context.Context) error {
	s.sessionsMu.Lock()
	for sess := range s.sessions {
		go sess.CloseGoingAway()
	}
	s.sessionsMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.conns.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		log.Println("Timed out waiting for connections to close")
	}
	s.stopWorld()
	return s.store.Close()
}

func (s *Server) join(sess *netconn.Session) {
	s.sessionsMu.Lock()
	s.sessions[sess] = true
	s.sessionsMu.Unlock()
	s.world.Join(sess, sess.PlayerID, sess.Username)
}

func (s *Server) leave(sess *netconn.Session) {
	s.sessionsMu.Lock()
	delete(s.sessions, sess)
	s.sessionsMu.Unlock()
	// The world releases the player's online claim
	s.world.Leave(sess)
}

// Claims the player's online slot. If they're already connected, e.g. a
// reconnect while the server still holds their dead connection, the old
// session is closed and this one takes its place.
func (s *Server) takeOver(playerID int) bool {
	if s.world.ClaimOnline(playerID) {
		return true
	}
	s.sessionsMu.Lock()
	for sess := range s.sessions {
		if sess.PlayerID == playerID {
			go sess.CloseWith(netconn.CloseReplaced, "logged in somewhere else")
		}
	}
	s.sessionsMu.Unlock()

	// The old session releases the slot once the world has removed it
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		if s.world.ClaimOnline(playerID) {
			return true
		}
	}
	return false
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Authenticate before upgrading, the id comes from the token not the client
	p, ok := s.auth.PlayerFromToken(r.URL.Query().Get("token"))
	if !ok {
		http.Error(w, "Invalid token.", http.StatusUnauthorized)
		return
	}
	if !s.takeOver(p.ID) {
		http.Error(w, "Already connected.", http.StatusConflict)
		return
	}

	s.conns.Add(1)
	defer s.conns.Done()

	err := netconn.Serve(w, r, p.ID, p.Username, s.router, s.join, s.leave)
	if err != nil {
		log.Println("Connection failed at Upgrader: ", err)
		s.world.ReleaseOnline(p.ID)
	}
}
