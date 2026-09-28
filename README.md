# nytrpg

A multiplayer browser RPG where the gameplay is built around daily NYT-style puzzles.

## Features

- **Real-time multiplayer** — move around a shared world and chat with other players
- **Daily puzzles** — Wordle with leaderboard rankings by guess count and solve time
- **Persistent accounts** — sign up, log in, and track your daily results

## Planned

- RPG classes (knight, cleric, rogue) with leveling and special abilities
- Connections and Mini Crossword puzzles
- Duels and quests

## Stack

- **Backend:** Go, SQLite, WebSocket (gorilla)
- **Frontend:** TypeScript, HTML5 Canvas

## Layout

```
cmd/server             entry point: config, graceful shutdown
cmd/bots               load-testing bots
cmd/protogen           generates client/src/protocol.gen.ts
internal/protocol      every websocket message (the source of truth)
internal/netconn       a player's websocket: reader/writer, pings, limits, message router
internal/game          the world: tick loop, entities, spatial grid, movement rules, replication
internal/game/maps     map JSON (bounds, spawn, clickable things)
internal/puzzles/...   daily puzzles (wordle)
internal/auth          accounts and JWTs
internal/store         SQLite and migrations
internal/server        wires it all together
client/src             TypeScript client
```

## Adding a feature

- **New message:** add the type and payload struct to `internal/protocol/protocol.go`, then
  `go generate ./internal/protocol` to update the client types (a test fails if you forget).
  Register a handler with `router.Handle(...)` in the feature's package, and `conn.on(...)` on the client.
- **Game logic:** world state lives on one goroutine. Change it from handlers with `world.Do(...)`,
  read it with `world.Query(...)`, and put per-tick logic in a system (`world.AddSystem`).
  Entities are replicated to nearby players automatically.
- **Rate limit an action:** `session.Allow("name", perSecond, burst)`.
- **Schema change:** append a migration to `internal/store/store.go`, never edit a shipped one.

## Testing

```bash
make test           # Go (with -race) + client unit tests, a few seconds. Run before every commit.
make test-browser   # headless Chromium end to end: login, movement, chat, wordle, reconnect
make check          # everything, as CI runs it
```

- **World rules** are unit tested with a fake clock (`internal/game`).
- **Anything sent over the websocket** gets an integration test against a real in-process server: `internal/testkit` signs players up, connects them, and waits for messages.
- **Client logic** is tested in Node (`client/test`), and the UI in a real browser (`client/test/browser`).

New features and bug fixes come with tests. The `nytrpg-testing` Claude Code skill (`.claude/skills/`) describes where each kind of test goes, with templates.

## Running

Locally:

```bash
npm install
npx tsc && JWT_SECRET=devsecret go run ./cmd/server
go run ./cmd/bots --n 10   # optional, in another terminal
```

Settings come from the environment: `JWT_SECRET` (required), `PORT` (8080), `DB_PATH` (`db/nytrpg.db`), `STATIC_DIR` (`client/static`), `DEBUG=1` (debug logs and `/debug/pprof`), `DUEL_WORD` (every duel uses this word, for end-to-end tests), `ANTHROPIC_API_KEY` (lets the gods answer Clerics' prayers through the Claude API; without it prayers are refunded), `PRAYER_FAKE=1` (canned prayer answers, no network, for end-to-end tests).

Metrics are at `/debug/vars`. To load test: `go run ./cmd/bots --n 200 --stagger 20ms --duration 30s --quiet`
prints throughput, bandwidth per bot, the largest gap between updates, and disconnects.

With Docker:

Uses a mount point, the db is written to disk outside of the container. `JWT_SECRET` is required, the server won't start without it.

```bash
docker build -t nytrpg .
docker run -p 8080:8080 -e JWT_SECRET=<some long random string> -v $(pwd)/db:/nytrpg/db nytrpg
```

### Deploying to EC2

`docker-compose.yml` runs the game behind Caddy, which serves HTTPS with a Let's Encrypt
certificate when `SITE_ADDRESS` is a domain (or plain HTTP on `:80`) and hides `/debug/*`.

1. Launch an instance (Amazon Linux 2023 or Ubuntu, t3.small or larger; a micro works with the swap
   the setup script adds). Security group: inbound 22 from your IP, 80 and 443 from anywhere.
   Attach an Elastic IP so the address survives a stop/start, and point your domain's A record at it.
2. On the instance: `curl -fsSL https://raw.githubusercontent.com/shantz14/nytrpg/main/deploy/ec2-setup.sh | bash`,
   log out and back in, fill in `~/nytrpg/.env` (see `.env.example`), then `cd ~/nytrpg && docker compose up -d --build`.
3. Later deploys, from your machine after pushing to main: `EC2_HOST=ec2-user@<ip> deploy/update.sh`.

Logs: `docker compose logs -f app`. The database lives in `~/nytrpg/db`, so back that directory up.
