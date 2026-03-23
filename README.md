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

## Running

```bash
docker build -t nytrpg .
docker run -p 8080:8080 nytrpg
```
