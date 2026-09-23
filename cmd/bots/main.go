package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"nytrpg/internal/protocol"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRes struct {
	ValidUser bool   `json:"validUser"`
	Id        int    `json:"id"`
	Jwt       string `json:"jwt"`
}

var chatLines = []string{
	"hello!",
	"anyone here?",
	"lol",
	"gg",
	"where is everyone",
	"hi",
	"nice",
	"what's up",
	"look at me go",
	"beep boop",
	"has anyone done the wordle yet?",
	"this game is cool",
	"brb",
}

func signup(baseURL, username, password string) {
	body, _ := json.Marshal(credentials{Username: username, Password: password})
	resp, err := http.Post(baseURL+"/signup", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

func login(baseURL, username, password string) (loginRes, error) {
	var res loginRes
	body, _ := json.Marshal(credentials{Username: username, Password: password})
	resp, err := http.Post(baseURL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return res, err
	}
	if !res.ValidUser {
		return res, fmt.Errorf("server rejected credentials")
	}
	return res, nil
}

func send(conn *websocket.Conn, t protocol.ClientMsg, data any) error {
	msg, err := protocol.EncodeClient(t, data)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, msg)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Stop reading and sending after connecting, like a client whose network died
var freeze = flag.Bool("freeze", false, "connect, then go silent (tests dead connection detection)")

func runBot(baseURL, wsBase, username, password string, wg *sync.WaitGroup) {
	defer wg.Done()

	signup(baseURL, username, password)

	res, err := login(baseURL, username, password)
	if err != nil {
		log.Printf("[%s] login failed: %v", username, err)
		return
	}
	id := res.Id
	log.Printf("[%s] logged in (id=%d)", username, id)

	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/ws?token=%s", wsBase, url.QueryEscape(res.Jwt)), nil)
	if err != nil {
		log.Printf("[%s] ws connect failed: %v", username, err)
		return
	}
	defer conn.Close()
	log.Printf("[%s] connected", username)

	if *freeze {
		log.Printf("[%s] frozen", username)
		select {}
	}

	// Drain server messages so the connection stays healthy.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	x, y := rand.Float64()*2345, rand.Float64()*2345
	dx := (rand.Float64()*2 - 1) * 15
	dy := (rand.Float64()*2 - 1) * 15

	moveTicker := time.NewTicker(time.Second / 30)
	dirTicker := time.NewTicker(time.Duration(1+rand.Intn(4)) * time.Second)
	chatTicker := time.NewTicker(time.Duration(15+rand.Intn(30)) * time.Second)

	for {
		select {
		case <-dirTicker.C:
			dx = (rand.Float64()*2 - 1) * 15
			dy = (rand.Float64()*2 - 1) * 15

		case <-moveTicker.C:
			x = clamp(x+dx, 50, 2345)
			y = clamp(y+dy, 50, 2345)
			if err := send(conn, protocol.ClientMove, protocol.Vec{X: int32(x), Y: int32(y)}); err != nil {
				log.Printf("[%s] send error: %v", username, err)
				return
			}

		case <-chatTicker.C:
			msg := chatLines[rand.Intn(len(chatLines))]
			if err := send(conn, protocol.ClientChat, protocol.ChatReq{Msg: msg}); err != nil {
				log.Printf("[%s] chat error: %v", username, err)
				return
			}
			log.Printf("[%s] says: %s", username, msg)
			chatTicker.Reset(time.Duration(15+rand.Intn(30)) * time.Second)
		}
	}
}

func main() {
	n := flag.Int("n", 5, "number of bots to spawn")
	server := flag.String("server", "http://localhost:8080", "server base URL")
	flag.Parse()

	wsBase := strings.Replace(*server, "http://", "ws://", 1)
	wsBase = strings.Replace(wsBase, "https://", "wss://", 1)

	log.Printf("Spawning %d bots against %s", *n, *server)

	var wg sync.WaitGroup
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go runBot(*server, wsBase, fmt.Sprintf("bot%d", i), "botpassword123", &wg)
		time.Sleep(150 * time.Millisecond) // stagger connects to avoid thundering herd
	}
	wg.Wait()
}
