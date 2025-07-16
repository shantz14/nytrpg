package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

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
	UsernameTaken bool `json:"usernameTaken"`
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
		log.Println("Error decoding login request:", err)
		return
	}
	var res SignupRes

	sql := `
	SELECT username FROM Player
	WHERE username = ?
	`
	rows, err := h.db.pool.Query(sql, req.Username)
	if err != nil {
		log.Fatal(err)
	}
	if (rows.Next()) {
		log.Println("Username already taken")
		res.UsernameTaken = true
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println("Error encoding signup response:", err)
		}
		return
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error hashing password:", err)
		return
	}
	hash := string(hashBytes)

	db := h.db
	sql = `
	INSERT INTO Player (username, password)
	VALUES (?, ?);
	`
	db.pool.Exec(sql, req.Username, hash)

	res.UsernameTaken = false
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error encoding signup response:", err)
		return
	}
}

func handleLogin(h *Hub, w http.ResponseWriter, r *http.Request) {
	var res UserData
	res.ValidUser = false
	res.Jwt = ""
	res.Id = -999
	res.Username = ""
	defer func() {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println("Error encoding login response:", err)
		}
	}()
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

	// Check against DB
	sql := `
	SELECT player_id, username, password FROM Player
	WHERE username = ?
	`

	rows, err := h.db.pool.Query(sql, req.Username)
	if err != nil {
		log.Println("Broke db login thing:", err)
		return
	}
	if (!rows.Next()) {
		log.Println("Invalid credentials (username)")
		return
	}
	var id int
	var username string
	var storedPass string
	err = rows.Scan(&id, &username, &storedPass)
	if err != nil {
		log.Println("Error scanning player row:", err)
		return
	}
	if (!checkPassword(req.Password, storedPass)) {
		log.Println("Invalid credentials (password)")
		return
	}

	if h.state.Players[id] != nil {
		log.Println("Already logged in")
		return
	}

	jwtStr, err := createToken(req.Username, req.Password)

	res.ValidUser = true
	res.Jwt = jwtStr
	res.Id = id
	res.Username = username
}

func checkPassword(password string, storedHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
    return err == nil
}

func createToken(uname string, psw string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, 
	jwt.MapClaims{ 
		"username": uname, 
		"password": psw,
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
		log.Println("Error decoding login request:", err)
		return
	}
	var res UserData
	if verified, uname := verifyToken(req); verified {
		row, found := h.db.getPlayerByUname(uname) 
		if !found {
			http.Error(w, "Couldn't find player row by username.", http.StatusInternalServerError)
			return
		}
		if h.state.Players[row.id] != nil {
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
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error encoding login response:", err)
		return
	}
}

func verifyToken(tokenStr string) (bool, string) {
	if tokenStr == "" {
		return false, ""
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		log.Println("Invalid jwt:", err)
		return false, ""
	}
	if !token.Valid {
		log.Println("Invalid jwt.")
		return false, ""
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		username := claims["username"].(string)
		return true, username
	}
	return false, ""
}
