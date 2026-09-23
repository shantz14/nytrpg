// Package auth handles accounts, passwords, and JWTs.
package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"nytrpg/internal/store"
)

const MaxUsernameLen = 20

// Logging in doesn't check whether the player is already connected: a new
// websocket connection replaces the old one, so there's still only ever one.
type Service struct {
	store  *store.Store
	secret []byte
}

func New(s *store.Store, secret []byte) *Service {
	return &Service{store: s, secret: secret}
}

type loginData struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserData struct {
	ValidUser bool   `json:"validUser"`
	Id        int    `json:"id"`
	Jwt       string `json:"jwt"`
	Username  string `json:"username"`
}

type signupRes struct {
	UsernameAvailable bool `json:"usernameAvailable"`
}

var invalidUser = UserData{ValidUser: false, Id: -999}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "err", err)
	}
}

func (a *Service) HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var req loginData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if n := utf8.RuneCountInString(req.Username); n == 0 || n > MaxUsernameLen {
		http.Error(w, "Username must be 1-20 characters.", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "Password required.", http.StatusBadRequest)
		return
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		http.Error(w, "Could not hash password.", http.StatusInternalServerError)
		slog.Error("hashing password", "err", err)
		return
	}

	taken, err := a.store.InsertPlayer(req.Username, string(hashBytes))
	if err != nil {
		http.Error(w, "Database error.", http.StatusInternalServerError)
		slog.Error("inserting player", "err", err)
		return
	}

	writeJSON(w, signupRes{UsernameAvailable: !taken})
}

func (a *Service) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var req loginData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, hash, found, err := a.store.PlayerAuth(strings.TrimSpace(req.Username))
	if err != nil {
		http.Error(w, "Database error.", http.StatusInternalServerError)
		slog.Error("looking up player", "err", err)
		return
	}
	if !found || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		writeJSON(w, invalidUser)
		return
	}
	jwtStr, err := a.CreateToken(p.Username)
	if err != nil {
		http.Error(w, "Could not create token.", http.StatusInternalServerError)
		slog.Error("creating token", "err", err)
		return
	}

	writeJSON(w, UserData{ValidUser: true, Jwt: jwtStr, Id: p.ID, Username: p.Username})
}

// Exchanges a saved JWT for the player's data
func (a *Service) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var token string
	if err := json.NewDecoder(r.Body).Decode(&token); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, ok := a.PlayerFromToken(token)
	if !ok {
		writeJSON(w, invalidUser)
		return
	}
	writeJSON(w, UserData{ValidUser: true, Jwt: token, Id: p.ID, Username: p.Username})
}

// Verifies a token and looks up the player it belongs to
func (a *Service) PlayerFromToken(token string) (store.Player, bool) {
	uname, ok := a.VerifyToken(token)
	if !ok {
		return store.Player{}, false
	}
	p, found, err := a.store.PlayerByUsername(uname)
	if err != nil {
		slog.Error("looking up player", "err", err)
		return p, false
	}
	return p, found
}

func (a *Service) CreateToken(uname string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": uname,
		"exp":      time.Now().Add(time.Hour * 72).Unix(),
	})
	return token.SignedString(a.secret)
}

// Returns the username in a valid token
func (a *Service) VerifyToken(tokenStr string) (string, bool) {
	if tokenStr == "" {
		return "", false
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return "", false
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if username, ok := claims["username"].(string); ok {
			return username, true
		}
	}
	return "", false
}
