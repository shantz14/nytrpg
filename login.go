package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var secretKey = []byte(os.Getenv("JWT_SECRET"))

type LoginData struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserData struct {
	ValidUser bool `json:"validUser"`
	Id int `json:"id"`
	Jwt string `json:"jwt"`
	Username string `json:"username"`
}

type SignupRes struct {
	UsernameAvailable bool `json:"usernameAvailable"`
}

const MAX_USERNAME_LEN = 20

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("Error encoding response:", err)
	}
}

func handleSignup(h *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var req LoginData
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println("Error decoding signup request:", err)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if n := utf8.RuneCountInString(req.Username); n == 0 || n > MAX_USERNAME_LEN {
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
		log.Println("Error hashing password:", err)
		return
	}

	taken, err := h.db.insertPlayer(req.Username, string(hashBytes))
	if err != nil {
		http.Error(w, "Database error.", http.StatusInternalServerError)
		log.Println("Error inserting player:", err)
		return
	}
	if taken {
		log.Println("Username already taken")
	}

	writeJSON(w, SignupRes{UsernameAvailable: !taken})
}

func handleLogin(h *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var req LoginData
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println("Error decoding login request:", err)
		return
	}

	invalid := UserData{ValidUser: false, Id: -999}

	id, username, storedPass, found, err := h.db.getPlayerAuth(strings.TrimSpace(req.Username))
	if err != nil {
		http.Error(w, "Database error.", http.StatusInternalServerError)
		log.Println("Broke db login thing:", err)
		return
	}
	if !found {
		log.Println("Invalid credentials (username)")
		writeJSON(w, invalid)
		return
	}
	if !checkPassword(req.Password, storedPass) {
		log.Println("Invalid credentials (password)")
		writeJSON(w, invalid)
		return
	}

	if h.isOnline(id) {
		log.Println("Already logged in")
		writeJSON(w, invalid)
		return
	}

	jwtStr, err := createToken(username)
	if err != nil {
		http.Error(w, "Could not create token.", http.StatusInternalServerError)
		log.Println("Error creating token:", err)
		return
	}

	writeJSON(w, UserData{ValidUser: true, Jwt: jwtStr, Id: id, Username: username})
}

func checkPassword(password string, storedHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
    return err == nil
}

func createToken(uname string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, 
	jwt.MapClaims{ 
		"username": uname, 
		"exp": time.Now().Add(time.Hour * 72).Unix(), 
	})

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func checkToken(h *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var req string
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println("Error decoding token request:", err)
		return
	}
	var res UserData
	if verified, uname := verifyToken(req); verified {
		row, found := h.db.getPlayerByUname(uname) 
		if !found {
			// Token for a player that doesn't exist (anymore), make them log in
			log.Println("Couldn't find player row by username.")
			res.Id = -999
			res.ValidUser = false
		} else if h.isOnline(row.id) {
			log.Println("Already logged in")
			res.Id = -999
			res.Username = ""
			res.Jwt = ""
			res.ValidUser = false
		} else {
			res.Id = row.id
			res.Username = row.username
			res.Jwt = req
			res.ValidUser = true
		}
	} else {
		res.Id = -999
		res.Username = ""
		res.Jwt = ""
		res.ValidUser = false
	}
	writeJSON(w, res)
}

func verifyToken(tokenStr string) (bool, string) {
	if tokenStr == "" {
		return false, ""
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return secretKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		log.Println("Invalid jwt:", err)
		return false, ""
	}
	if !token.Valid {
		log.Println("Invalid jwt.")
		return false, ""
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if username, ok := claims["username"].(string); ok {
			return true, username
		}
	}
	return false, ""
}
