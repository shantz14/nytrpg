package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"

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

// The bot's character: the one in slot 0, made with a random class if needed
func character(baseURL, jwt, username string) (int, error) {
	do := func(method string, body any, out any) error {
		var r io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			r = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, baseURL+"/characters", r)
		req.Header.Set("Authorization", "Bearer "+jwt)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s /characters: %s", method, resp.Status)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}

	var list struct {
		Slots   []*protocol.CharacterInfo `json:"slots"`
		Classes []protocol.ClassInfo      `json:"classes"`
	}
	if err := do(http.MethodGet, nil, &list); err != nil {
		return 0, err
	}
	if len(list.Slots) > 0 && list.Slots[0] != nil {
		return list.Slots[0].ID, nil
	}
	if len(list.Classes) == 0 {
		return 0, fmt.Errorf("server has no classes")
	}
	class := list.Classes[rand.Intn(len(list.Classes))].ID
	var c protocol.CharacterInfo
	err := do(http.MethodPost, map[string]any{"slot": 0, "name": username, "class": class}, &c)
	return c.ID, err
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

var (
	// Stop reading and sending after connecting, like a client whose network died
	freeze   = flag.Bool("freeze", false, "connect, then go silent (tests dead connection detection)")
	quiet    = flag.Bool("quiet", false, "only log errors and the summary")
	duration = flag.Duration("duration", 0, "stop after this long and print a summary (0 = run forever)")
	stagger  = flag.Duration("stagger", 150*time.Millisecond, "delay between bot connects")
)

const (
	// Same as the real client
	moveSpeed = 450.0 // px/s
	sendRate  = 20    // Hz
)

// Set once every bot has connected; traffic before that isn't counted
var measuring atomic.Bool

// What one bot saw, merged into the summary when it stops
type botStats struct {
	msgs, bytes int64
	maxGap      time.Duration
	corrections int64
	// Connection ended before the run did
	dropped bool
}

type summary struct {
	mu        sync.Mutex
	bots      []botStats
	connected int
}

func (s *summary) add(b botStats) {
	s.mu.Lock()
	s.bots = append(s.bots, b)
	s.mu.Unlock()
}

func logf(format string, args ...any) {
	if !*quiet {
		log.Printf(format, args...)
	}
}

func runBot(ctx context.Context, baseURL, wsBase, username, password string, sum *summary, wg *sync.WaitGroup) {
	defer wg.Done()

	signup(baseURL, username, password)

	res, err := login(baseURL, username, password)
	if err != nil {
		log.Printf("[%s] login failed: %v", username, err)
		return
	}
	logf("[%s] logged in (id=%d)", username, res.Id)

	charID, err := character(baseURL, res.Jwt, username)
	if err != nil {
		log.Printf("[%s] no character: %v", username, err)
		return
	}

	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/ws?token=%s&character=%d", wsBase, url.QueryEscape(res.Jwt), charID), nil)
	if err != nil {
		log.Printf("[%s] ws connect failed: %v", username, err)
		return
	}
	defer conn.Close()
	sum.mu.Lock()
	sum.connected++
	sum.mu.Unlock()
	logf("[%s] connected", username)

	if *freeze {
		logf("[%s] frozen", username)
		<-ctx.Done()
		return
	}

	// Read everything the server sends, measuring it. Where we are comes from the
	// server: the spawn point in the welcome, and any corrections.
	readerDone := make(chan botStats, 1)
	setPos := make(chan protocol.Vec, 8)
	go func() {
		var st botStats
		var last time.Time
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				st.dropped = ctx.Err() == nil
				readerDone <- st
				return
			}
			if t, data, err := protocol.Decode(msg); err == nil {
				switch protocol.ServerMsg(t) {
				case protocol.ServerWelcome:
					var w protocol.Welcome
					if msgpack.Unmarshal(data, &w) == nil {
						setPos <- w.Pos
					}
				case protocol.ServerCorrection:
					var v protocol.Vec
					if msgpack.Unmarshal(data, &v) == nil {
						st.corrections++
						setPos <- v
					}
				}
			}
			if !measuring.Load() {
				continue
			}
			now := time.Now()
			if !last.IsZero() && now.Sub(last) > st.maxGap {
				st.maxGap = now.Sub(last)
			}
			last = now
			st.msgs++
			st.bytes += int64(len(msg))
		}
	}()

	var x, y float64
	select {
	case p := <-setPos:
		x, y = float64(p.X), float64(p.Y)
	case <-ctx.Done():
		conn.Close()
		sum.add(<-readerDone)
		return
	}
	newDir := func() (float64, float64) {
		angle := rand.Float64() * 2 * math.Pi
		step := moveSpeed / sendRate
		return math.Cos(angle) * step, math.Sin(angle) * step
	}
	dx, dy := newDir()

	moveTicker := time.NewTicker(time.Second / sendRate)
	dirTicker := time.NewTicker(time.Duration(1+rand.Intn(4)) * time.Second)
	chatTicker := time.NewTicker(time.Duration(15+rand.Intn(30)) * time.Second)
	defer moveTicker.Stop()
	defer dirTicker.Stop()
	defer chatTicker.Stop()

	var sendErr error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case p := <-setPos:
			x, y = float64(p.X), float64(p.Y)

		case <-dirTicker.C:
			dx, dy = newDir()

		case <-moveTicker.C:
			x = clamp(x+dx, 50, 2345)
			y = clamp(y+dy, 50, 2345)
			sendErr = send(conn, protocol.ClientMove, protocol.Vec{X: int32(x), Y: int32(y)})

		case <-chatTicker.C:
			msg := chatLines[rand.Intn(len(chatLines))]
			sendErr = send(conn, protocol.ClientChat, protocol.ChatReq{Msg: msg})
			logf("[%s] says: %s", username, msg)
			chatTicker.Reset(time.Duration(15+rand.Intn(30)) * time.Second)
		}
		if sendErr != nil {
			log.Printf("[%s] send error: %v", username, sendErr)
			break
		}
	}

	conn.Close()
	sum.add(<-readerDone)
}

func (s *summary) print(n int, elapsed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bots) == 0 {
		fmt.Println("no bots finished")
		return
	}
	var msgs, bytes, corrections int64
	dropped := 0
	gaps := make([]time.Duration, 0, len(s.bots))
	for _, b := range s.bots {
		msgs += b.msgs
		bytes += b.bytes
		gaps = append(gaps, b.maxGap)
		corrections += b.corrections
		if b.dropped {
			dropped++
		}
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	secs := elapsed.Seconds()
	perBot := float64(bytes) / float64(len(s.bots)) / secs

	fmt.Printf("\n--- %d bots, %d connected, %s ---\n", n, s.connected, elapsed.Round(time.Second))
	fmt.Printf("received: %.0f msgs/s total, %.1f KB/s per bot (%.1f MB/s total)\n", float64(msgs)/secs, perBot/1024, float64(bytes)/secs/1024/1024)
	fmt.Printf("largest gap between messages per bot: p50 %s, p99 %s, max %s\n",
		gaps[len(gaps)/2].Round(time.Millisecond), gaps[len(gaps)*99/100].Round(time.Millisecond), gaps[len(gaps)-1].Round(time.Millisecond))
	fmt.Printf("disconnected early: %d, moves corrected by the server: %d\n", dropped, corrections)
}

func main() {
	n := flag.Int("n", 5, "number of bots to spawn")
	server := flag.String("server", "http://localhost:8080", "server base URL")
	flag.Parse()

	wsBase := strings.Replace(*server, "http://", "ws://", 1)
	wsBase = strings.Replace(wsBase, "https://", "wss://", 1)

	log.Printf("Spawning %d bots against %s", *n, *server)

	// Ctrl-C stops the bots and prints the summary
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithCancel(sigCtx)

	sum := &summary{}
	var wg sync.WaitGroup
	for i := 0; i < *n && ctx.Err() == nil; i++ {
		wg.Add(1)
		go runBot(ctx, *server, wsBase, fmt.Sprintf("bot%d", i), "botpassword123", sum, &wg)
		time.Sleep(*stagger) // stagger connects to avoid thundering herd
	}

	// Measure once everyone is connected
	measuring.Store(true)
	start := time.Now()
	var timeout <-chan time.Time
	if *duration > 0 {
		timeout = time.After(*duration)
	}
	select {
	case <-sigCtx.Done():
	case <-timeout:
	}
	cancel()
	wg.Wait()
	sum.print(*n, time.Since(start))
}
