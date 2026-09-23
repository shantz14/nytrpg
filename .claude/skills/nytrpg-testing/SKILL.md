---
name: nytrpg-testing
description: Testing workflow for the nytrpg game (Go server + TypeScript canvas client). Use this whenever you add or change a feature, fix a bug, add or change a websocket message, touch internal/game, netconn, protocol, store, auth, puzzles, or client/src in nytrpg, even if the user never mentions tests. It says which tests to write, where they go, which helpers to use, and what must pass before the work counts as done. Also use it when asked to review, extend, debug or speed up nytrpg's test suite.
---

# Testing nytrpg

Lots of real-time, multiplayer features are going into this game. Those break in ways nobody notices by clicking around: races, ghost players, desyncs, cheating clients. So the rule is simple: **a change isn't done until tests that would catch it breaking exist and pass.** Every test in the suite so far has either caught a real bug or pinned down a fix, so this pays off.

## Workflow

1. **Bug fix? Write the failing test first.** Reproduce the bug in a test, watch it fail for the right reason, then fix it. The test stays as the regression test; say so in a comment ("used to ...").
2. **Feature? Decide the layers** with the table below. Most features need a unit test *and* an integration test. Add a browser test when the UI changes.
3. **Protocol change?** Edit `internal/protocol/protocol.go`, then run `make generate`. `TestGeneratedProtocolIsCurrent` fails if you forget.
4. **Run `make test`** (Go with `-race`, plus client unit tests, a few seconds). Run `make test-browser` when you changed client UI or anything a player sees. Both must pass. Don't skip, delete or weaken an existing test to get there. If one really is wrong, fix it and say why.
5. **Report** which tests you added (file and name, and what each proves) and the commands you ran with their results.

## Where tests go

| What changed | Test layer | Where | Use |
|---|---|---|---|
| World rules: movement, entities, visibility, per-tick systems, chat routing | World unit test, fake clock, no goroutines | `internal/game/*_test.go` | `newTestWorld()`, `tw.join(id, pos)`, `tw.tickNow()`, `tw.advance(d)`, `fakeClient.updates(t)` / `.take(type)` |
| A websocket message or anything a client can send | Integration test, real server over websockets | `internal/server/server_test.go` (or a new `_test.go` there) | `testkit.NewServer`, `ts.Connect`, `c.Send`, `testkit.Expect[T]`, `c.WatchWorld`, `c.WalkTo`, `c.ExpectNone`, `c.Count` |
| Connection handling: limits, routing, close codes | netconn unit test | `internal/netconn/session_test.go` | `serve(t, router)` helper; `newSession` to test without a writer |
| SQL, schema, migrations | Store test on a temp DB | `internal/store/store_test.go` | `open(t)`. **Schema changes append a migration**, never edit a shipped one. |
| HTTP handlers (auth, leaderboard, new endpoints) | `httptest` recorder or integration | `internal/auth/auth_test.go`, `server_test.go` | `call(handler, method, body)`, `ts.PostJSON` / `ts.GetJSON` |
| Puzzle logic (scoring, sessions) | Unit test, plus service test on a temp DB | `internal/puzzles/<name>/*_test.go` | fake `inRange` func, `store.Open(t.TempDir())` |
| Client logic that doesn't need the DOM: interpolation, math, `Connection` | Node unit test | `client/test/*.test.mjs` | `fakeClock()` from `setup.mjs`, `FakeSocket` in `net.test.mjs`, `mock.timers` |
| Anything a player sees or clicks: popups, input, rendering flow, reconnect | Browser test (headless Chromium) | `client/test/browser/run.mjs` | `player()`, `toScreen`, `hold`, `received(page, type)`, `waitFor`, `visible` |

Copy-ready templates for each layer are in `references/templates.md`. Read it when writing a kind of test you haven't written here yet.

## What a good test here checks

- **The rule, not just the happy path.** For anything a client sends, also test the hostile version: wrong owner, too fast, too far, too often, malformed, after the game is over. The server must never trust the client (ids, counts, times, positions), and a test should prove it.
- **What other players see.** Multiplayer bugs show up on the *other* client: did they get the spawn, the move, the despawn, the chat? Is it withheld from players out of view?
- **Reconnects and ordering.** State that must survive a reconnect (like a Wordle in progress) gets a disconnect/redial in the test. Messages that can cross in flight get a test that sends them back to back (the Wordle "submit before the server's reply" bug was exactly this).
- **Limits.** New actions that players can spam get a `session.Allow` rate limit, and a test that floods them.

## Pitfalls this suite already hit

- **Don't sleep to wait for game logic.** In world tests, drive time with `tw.advance` and `tw.tickNow()`. In integration tests, wait with `Expect`/`WatchWorld`/`Count`, which return as soon as the message arrives. Sleeps are only for pacing a real client (`WalkTo` does this for you).
- **Moves must be legal.** The server corrects anything faster than `game.MoveSpeed` (with a small budget) or off the map. Move players in tests with `c.WalkTo(t, target, game.MoveSpeed)` or small steps, never by jumping, unless the test is about teleport rejection.
- **Players only see nearby entities** (3x3 cells of 1024px). Put players who need to see each other close together; `testkit` spawns everyone near the map's spawn point.
- **Chat is rate limited** (burst 3, then 1/s), including in tests. Count chats you already sent when asserting on a flood.
- **The first world tick after joining** is when players learn about each other. Wait for it (`WatchWorld`, or one `2*time.Second/game.TickRate` pause) before expecting chats between them.
- **Process-wide counters** (`netconn.Stats`, `/debug/vars`) are shared across tests. Assert on deltas (`before := ...; after-before == 1`), and keep tests that read `/debug/vars` sequential (no `t.Parallel`).
- **Integration tests use `t.Parallel()`**, since each test has its own server and database. Keep it that way.
- **gorilla/websocket:** after a read deadline fires the connection is dead; don't reuse it. The testkit client reads on its own goroutine for this reason.
- **msgpack:** slices of `uint8`-based types encode as binary (the browser gets a `Uint8Array`), so enums used in slices are `int`. Maps with int keys don't decode into `map[string]any`. Decode into the protocol structs.
- **Browser tests: wait on the DOM, not on frames.** The DevTools frame event fires before the page's JS handles the message. Use `waitFor(() => page.$eval(...))`.
- **Imports:** `testkit` imports the whole server, so only tests *outside* `internal/server`'s dependencies (e.g. `package server_test`) can use it. Package-internal tests of `store`, `wordle`, `game` etc. use their own small helpers.

## Commands

```bash
make test           # go vet + go test -race ./... + client unit tests (seconds). Before every commit.
make test-browser   # headless Chromium end to end (~30s). When the UI or player-visible behavior changed.
make check          # everything, as CI runs it (.github/workflows/test.yml)
make generate       # after changing internal/protocol
make bench          # world tick cost and bandwidth per client, when touching replication or the tick
go test -race -run TestName ./internal/server/   # one test while iterating
```

If a test is flaky, don't retry until it's green. Flakiness here has meant a real race (in the code or the test) every time so far. Run it with `-count=20` (Go) or in a loop (browser) until you understand why, then fix the cause.
