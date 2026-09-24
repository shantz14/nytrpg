// Package game is the shared, real-time world: entities, the tick loop, movement
// rules, and replicating what each player can see.
//
// All world state is owned by one goroutine (Run). Everything else talks to it by
// sending commands, so game code never needs locks. To add a feature, either send
// a command from a message handler (w.Do) or add a system that runs every tick
// (w.AddSystem).
package game

import (
	"context"
	"log/slog"
	"math/rand"
	"runtime/debug"
	"time"

	"nytrpg/internal/classes"
	"nytrpg/internal/protocol"
)

const (
	TickRate = 20 // Hz

	playerSprite = "Skoobyuboo.png"
	// Players spawn scattered this far around the map's spawn point
	spawnSpread = 100
)

// Something the world can send messages to, a player's connection
type Client interface {
	// Queues an encoded message, never blocks
	Send(msg []byte) bool
}

type Entity struct {
	ID   protocol.EntityID
	Kind protocol.EntityKind
	Name string
	// Players only, the character name and class ID
	Char   string
	Class  string
	Sprite string
	Pos    protocol.Vec

	// Moved since the last tick
	moved bool
	cell  cellKey
}

func (e *Entity) spawnMsg() protocol.EntitySpawn {
	return protocol.EntitySpawn{ID: e.ID, Kind: e.Kind, Name: e.Name, Char: e.Char, Class: e.Class, Sprite: e.Sprite, Pos: e.Pos}
}

type player struct {
	client   Client
	playerID int
	ent      *Entity
	// Entities this client knows about, and the tick they were last in view
	known map[protocol.EntityID]uint64
	moveBudget
	// The duel they're in, nil when not dueling
	duel *duel
}

// A system runs every tick, in the order added, before replication
type System func(w *World)

type World struct {
	presence

	Map *protocol.WorldMap
	// The puzzle duels are played on. Set before Run, nil turns duels off.
	Duels DuelPuzzle

	// Owned by the world goroutine
	entities map[protocol.EntityID]*Entity
	players  map[Client]*player
	grid     *grid
	nextID   protocol.EntityID
	tick     uint64
	moved    []*Entity
	systems  []System
	// Duel challenges waiting for an answer, by id
	challenges    map[uint32]*challenge
	nextChallenge uint32

	cmds chan func()
	// For tests
	now func() time.Time
	rng *rand.Rand

	Stats Stats
}

func NewWorld(m *protocol.WorldMap) *World {
	return &World{
		presence: presence{online: make(map[int]bool)},
		Map:      m,
		entities: make(map[protocol.EntityID]*Entity),
		players:  make(map[Client]*player),
		grid:     newGrid(),
		// Built in systems run before any added ones
		systems:    []System{(*World).expireChallenges},
		challenges: make(map[uint32]*challenge),
		cmds:       make(chan func(), 1024),
		now:        time.Now,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Runs the world until ctx is done
func (w *World) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-w.cmds:
			w.safely(cmd)
		case <-ticker.C:
			w.safely(w.step)
		}
	}
}

// Runs fn on the world goroutine. A panic is logged instead of stopping the world.
func (w *World) safely(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			w.Stats.Panics.Add(1)
			slog.Error("panic in world", "panic", r, "stack", string(debug.Stack()))
		}
	}()
	fn()
}

// Queues fn to run on the world goroutine
func (w *World) Do(fn func(w *World)) {
	w.cmds <- func() { fn(w) }
}

// Runs fn on the world goroutine and waits for it to finish
func (w *World) Query(fn func(w *World)) {
	done := make(chan struct{})
	w.cmds <- func() {
		defer close(done)
		fn(w)
	}
	<-done
}

// Adds a system that runs every tick. Call before Run.
func (w *World) AddSystem(s System) {
	w.systems = append(w.systems, s)
}

func (w *World) step() {
	start := time.Now()
	w.tick++
	for _, s := range w.systems {
		s(w)
	}
	w.replicate()
	w.Stats.recordTick(time.Since(start))
}

func (w *World) newEntity(kind protocol.EntityKind, name, sprite string, pos protocol.Vec) *Entity {
	w.nextID++
	e := &Entity{ID: w.nextID, Kind: kind, Name: name, Sprite: sprite, Pos: pos}
	w.entities[e.ID] = e
	w.grid.insert(e)
	return e
}

func (w *World) removeEntity(e *Entity) {
	w.grid.remove(e)
	delete(w.entities, e.ID)
}

// Call after changing e.Pos
func (w *World) entityMoved(e *Entity) {
	w.grid.moved(e)
	if !e.moved {
		e.moved = true
		w.moved = append(w.moved, e)
	}
}

func (w *World) send(c Client, t protocol.ServerMsg, data any) {
	msg, err := protocol.Encode(t, data)
	if err != nil {
		slog.Error("encoding message", "err", err)
		return
	}
	w.Stats.BytesOut.Add(int64(len(msg)))
	c.Send(msg)
}

// Adds a connected player to the world, playing the given character
func (w *World) Join(c Client, playerID int, username string, ch protocol.CharacterInfo) {
	w.Do(func(w *World) {
		spawn := w.Map.Spawn
		spawn.X = clamp(spawn.X+w.rng.Int31n(2*spawnSpread+1)-spawnSpread, 0, w.Map.Width)
		spawn.Y = clamp(spawn.Y+w.rng.Int31n(2*spawnSpread+1)-spawnSpread, 0, w.Map.Height)

		p := &player{
			client:   c,
			playerID: playerID,
			ent:      w.newEntity(protocol.EntityPlayer, username, playerSprite, spawn),
			known:    make(map[protocol.EntityID]uint64),
		}
		p.ent.Char = ch.Name
		p.ent.Class = ch.Class
		p.moveBudget.reset(w.now())
		w.players[c] = p

		w.send(c, protocol.ServerWelcome, protocol.Welcome{
			PlayerID:  playerID,
			Username:  username,
			EntityID:  p.ent.ID,
			Pos:       spawn,
			Map:       *w.Map,
			MoveSpeed: MoveSpeed,
			TickRate:  TickRate,
			Character: ch,
			Classes:   classes.Infos(),
		})
	})
}

// Removes a player. Everyone who could see them gets a despawn next tick.
func (w *World) Leave(c Client) {
	w.Do(func(w *World) {
		p, ok := w.players[c]
		if !ok {
			return
		}
		w.leaveDuels(p)
		w.removeEntity(p.ent)
		delete(w.players, c)
		w.ReleaseOnline(p.playerID)
	})
}

func clamp(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
