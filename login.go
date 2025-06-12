package main

import (
	"log"
	"os"
	"time"
	"net/http"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte(os.Getenv("JWT_SECRET"))

type LoginData struct {
	Username string
	Password string
}

type UserData struct {
	ValidUser bool
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

	log.Println(req.Username, req.Password)
	//jwtStr, err := createToken(req.username, req.password)

	var res UserData
	res.ValidUser = true
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error encoding login response:", err)
		return
	}
}

func createToken(uname string, psw string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, 
	jwt.MapClaims{ 
		"username": uname, 
		"password": psw,
		"exp": time.Now().Add(time.Hour * 24).Unix(), 
	})

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func verifyToken(tokenStr string) bool {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return os.Getenv("JWT_SECRET"), nil
	})
	if err != nil {
		log.Println("Error parsing jwt:", err)
		return false
	}
	if !token.Valid {
		log.Println("Invalid jwt.")
		return false
	}
	return true
}
