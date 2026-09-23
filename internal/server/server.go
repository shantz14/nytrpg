// Package server wires every feature together behind one http.Handler.
package server

import (
	"context"
	"expvar"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"sync"
	"sync/atomic"
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
		wordle:   wordle.NewService(st, func(s *netconn.Session) bool { return world.InRange(s, "wordle") }),
		router:   netconn.NewRouter(),
		sessions: make(map[*netconn.Session]bool),
	}

	// Each feature registers the websocket messages it handles
	s.world.RegisterHandlers(s.router)
	s.wordle.RegisterHandlers(s.router)

	s.publishMetrics()

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
	mux.HandleFunc("/leaderboard", s.wordle.HandleLeaderboard)

	// Metrics, as JSON. Counters only grow; diff two reads for rates.
	mux.Handle("/debug/vars", expvar.Handler())
	if s.cfg.Debug {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	return mux
}

var (
	publishOnce sync.Once
	// expvar names can only be published once per process, so the metrics
	// report whichever server was created last
	metricsServer atomic.Pointer[Server]
)

// Server metrics under "nytrpg" in /debug/vars
func (s *Server) publishMetrics() {
	metricsServer.Store(s)
	publishOnce.Do(func() {
		expvar.Publish("nytrpg", expvar.Func(func() any {
			s := metricsServer.Load()
			ws, cs := &s.world.Stats, &netconn.Stats
			ms := func(nanos int64) float64 { return float64(nanos) / 1e6 }
			avg := 0.0
			if t := ws.Ticks.Load(); t > 0 {
				avg = ms(ws.TotalTickNanos.Load()) / float64(t)
			}
			return map[string]any{
				"players_online":    s.world.Count(),
				"connections_open":  cs.Open.Load(),
				"connections_total": cs.Total.Load(),
				"ticks":             ws.Ticks.Load(),
				"tick_last_ms":      ms(ws.LastTickNanos.Load()),
				"tick_max_ms":       ms(ws.MaxTickNanos.Load()),
				"tick_avg_ms":       avg,
				"world_bytes_out":   ws.BytesOut.Load(),
				"rejected_moves":    ws.RejectedMoves.Load(),
				"world_panics":      ws.Panics.Load(),
				"slow_client_kicks": cs.SlowClientKicks.Load(),
				"flood_kicks":       cs.FloodKicks.Load(),
				"rate_limited_msgs": cs.RateLimited.Load(),
				"bad_msgs":          cs.BadMessages.Load(),
				"handler_panics":    cs.HandlerPanics.Load(),
			}
		}))
	})
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
		slog.Warn("timed out waiting for connections to close")
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
		slog.Info("websocket upgrade failed", "player", p.ID, "err", err)
		s.world.ReleaseOnline(p.ID)
	}
}
