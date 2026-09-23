# Test templates for nytrpg

Copy the section for the layer you need. Helper names and signatures here match
the code; if one stops matching, fix this file along with the code.

- [1. World logic (internal/game)](#1-world-logic)
- [2. A new websocket message, end to end (integration)](#2-a-new-websocket-message)
- [3. A per-tick system](#3-a-per-tick-system)
- [4. Schema change (internal/store)](#4-schema-change)
- [5. Connection behavior (internal/netconn)](#5-connection-behavior)
- [6. Client logic (Node)](#6-client-logic)
- [7. Browser (UI and what players see)](#7-browser)
- [8. Regression test for a bug](#8-regression-test-for-a-bug)

---

## 1. World logic

Synchronous and deterministic: nothing runs until you call `flush`/`tickNow`,
and time only moves when you call `advance`. Lives in `package game`, so it can
reach internals (`tw.players[c].ent.Pos`, `tw.grid`, ...).

```go
func TestSomethingInTheWorld(t *testing.T) {
	tw := newTestWorld()                            // 5000x5000 map, spawn (500,500), fake clock
	a, pa := tw.join(1, protocol.Vec{X: 100, Y: 100}) // returns *fakeClient, *player
	b, _ := tw.join(2, protocol.Vec{X: 200, Y: 100})
	tw.tickNow()                                    // run queued commands + one tick
	a.msgs, b.msgs = nil, nil                       // forget the initial spawns

	tw.advance(100 * time.Millisecond)
	tw.Move(b, protocol.Vec{X: 230, Y: 100})
	tw.tickNow()

	ups := a.updates(t) // decoded ServerWorld messages since last check
	if len(ups) != 1 || len(ups[0].Move) != 1 {
		t.Fatalf("a should see b move: %+v", ups)
	}
	_ = pa
	// Other message types: a.take(protocol.ServerChat) returns the raw payloads
}
```

Call `tw.flush()` to run queued commands *without* a tick, e.g. to check a
correction right after `tw.Move`.

## 2. A new websocket message

Steps for a feature like "players can emote":

1. `internal/protocol/protocol.go`: add `ClientEmote ClientMsg = N` / `ServerEmote ServerMsg = M` and the payload structs. Run `make generate`.
2. Handler in the feature's package: `r.Handle(protocol.ClientEmote, func(s *netconn.Session, data msgpack.RawMessage) {...})`. It unmarshals, rate limits with `s.Allow("emote", perSec, burst)`, and changes the world through `w.Do(...)`, never directly.
3. Register it in `internal/server/server.go` (`New`) if it's a new package.
4. Client: `conn.on<Emote>(ServerEmote, ...)` in `client/src/game.ts` or the feature's module.
5. Tests: a world or unit test for the rule (section 1), plus this integration test:

```go
func TestEmote(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	a := ts.Connect(t, "a")
	b := ts.Connect(t, "b")
	time.Sleep(2 * time.Second / game.TickRate) // first tick: they learn about each other

	a.Send(t, protocol.ClientEmote, protocol.EmoteReq{Kind: "wave"})
	got := testkit.Expect[protocol.EmoteMsg](t, b, protocol.ServerEmote)
	if got.ID != a.Welcome.EntityID {
		t.Fatalf("emote should come from a's entity: %+v", got)
	}

	// Hostile cases: whatever the client claims, the server decides
	a.Send(t, protocol.ClientEmote, protocol.EmoteReq{Kind: "not-a-real-emote"})
	b.ExpectNone(t, protocol.ServerEmote, 300*time.Millisecond)

	// Spam is limited
	for i := 0; i < 20; i++ {
		a.Send(t, protocol.ClientEmote, protocol.EmoteReq{Kind: "wave"})
	}
	if n := b.Count(protocol.ServerEmote, time.Second); n > 3 {
		t.Fatalf("emote spam not limited: %d got through", n)
	}

	// Players out of view don't get it
	far := ts.Connect(t, "far")
	far.WalkTo(t, protocol.Vec{X: far.Pos.X + 3000, Y: far.Pos.Y}, game.MoveSpeed) // slow (~7s); only when view matters
	a.Send(t, protocol.ClientEmote, protocol.EmoteReq{Kind: "wave"})
	far.ExpectNone(t, protocol.ServerEmote, 500*time.Millisecond)
}
```

Other testkit tools:
- `ts.Login(t, name)` returns an `Account`; `ts.Dial(t, acct)` connects as it again (reconnects, takeovers).
- `ts.DialRaw(token)` connects without waiting for the welcome (auth tests); it returns `(client, httpStatus, err)`.
- `c.WatchWorld(d, func(v testkit.View) bool {...})` folds world updates into `v.Pos`, `v.Names` and `v.Despawn` until the func returns true or `d` passes.
- `c.WaitClosed(t)` returns the close error; check codes with `websocket.IsCloseError(err, code)`.
- `c.SendRaw(t, bytes)` sends garbage.
- `ts.PostJSON(t, path, body, &out)` and `ts.GetJSON(t, path, &out)` return the HTTP status.
- `testkit.UniqueName("p")` gives a name no other test uses.

## 3. A per-tick system

Per-tick logic (cooldowns, regen, projectiles, NPC movement) is a `System`,
registered with `w.AddSystem(...)` before `Run`. It runs on the world goroutine,
so it can touch world state freely. Test it with the fake clock:

```go
func TestCooldownSystem(t *testing.T) {
	tw := newTestWorld()
	tw.AddSystem(cooldownSystem)          // the system under test
	c, p := tw.join(1, protocol.Vec{X: 100, Y: 100})
	_ = c
	// ... put p on cooldown ...
	for i := 0; i < TickRate; i++ {       // one simulated second
		tw.advance(time.Second / TickRate)
		tw.tickNow()
	}
	// assert on p's state, and on what clients were sent
}
```

If the system moves entities, call `w.entityMoved(e)` after changing `e.Pos`,
or nobody is told about the move.

## 4. Schema change

Append to `migrations` in `internal/store/store.go`; never edit a shipped entry
(existing databases already ran it). Then:

```go
func TestNewThing(t *testing.T) {
	s := open(t) // temp db, migrations applied
	// exercise the new Store methods, including not-found and duplicate cases
}
```

`TestMigrationsRunOnceAndSurviveReopen` already checks that migrations apply on
reopen. If the migration transforms existing data, add a test that opens a db
at the old version: insert rows, set `schema_version` back, reopen, and check them.

## 5. Connection behavior

`internal/netconn/session_test.go` has `serve(t, router)`, which returns a dial
func and a channel of joined sessions, plus `send` and `waitClosed`. To test
`Session` behavior without the writer goroutine (queue limits),
`TestSendNeverBlocksAndDropsSlowClients` shows how to build a bare session with
`newSession`.

## 6. Client logic

Node tests run against the compiled JS (`npm test` compiles first), so import
from `../static/<module>.js`. Only modules that don't touch the DOM when
imported can be loaded; pull pure logic into such modules (like
`game-objects.ts`) so it can be tested.

```js
import { test } from "node:test";
import assert from "node:assert/strict";
import { fakeClock } from "./setup.mjs";          // also sets globalThis.MessagePack
import { Thing } from "../static/thing.js";

test("thing does the right thing", () => {
    const clock = fakeClock(1000);                  // controls performance.now()
    try {
        clock.advance(50);
        assert.equal(new Thing().value(), 42);
    } finally {
        clock.restore();
    }
});
```

For code using timers: `mock.timers.enable({ apis: ["setTimeout"] })` and
`mock.timers.tick(ms)`. For code using the websocket: the `FakeSocket` in
`net.test.mjs` (`open()`, `receive(type, payload)`, `drop(code)`, `.sent`).

## 7. Browser

`client/test/browser/run.mjs` builds and starts a real server (temp db, free
port) and drives headless Chromium. Add a `test("...", async () => {...})`:

```js
test("players can emote", async () => {
    const a = await player();          // signed up, logged in, welcomed; a.pos = world position
    const b = await player();
    await a.keyboard.press("e");       // whatever the UI does
    await waitFor(() => received(b, SERVER_EMOTE).length, "b to get the emote");
    // Assert on the page, and wait for it: frames arrive before the page handles them
    await waitFor(() => b.$eval("#something", (e) => e.textContent === "wave"), "emote shown");
    await a.browserContext().close();
    await b.browserContext().close();
});
```

Helpers:
- `toScreen(page, worldX, worldY)` gives a click point.
- `hold(page, key, ms)` walks at 450px/s.
- `received(page, type)` and `page.frames` are the decoded websocket frames, `{dir: "in"|"out", t, d}`.
- `visible(page, selector)`, `waitFor(fn, what, timeout)`.
- `stopServer()` / `startServer()` restart the server (same db) for reconnect tests.
- `draws(page, sinceT, self)` returns the character sprites drawn on the canvas since page time `sinceT` (`pageNow(page)`), for your own player (`self = true`) or others: `{file, sx, mirrored, x}`. Use it for animation and rendering checks; to track a new sprite file, add it to `recordCharacterDraws`.
- World coordinates of interactables come from `internal/game/maps/town.json`.

Any page error or `console.error` fails the test, so fix those rather than ignoring them.

## 8. Regression test for a bug

1. Write the smallest test that shows the bug, at the lowest layer where it can be seen.
2. Run it and watch it fail *for the reason you think*. Print what actually happened if unsure.
3. Fix the code. Keep the test, with a comment saying what used to go wrong:

```go
// A token without a username claim used to panic the server (unchecked type assertion)
"no username": sign(jwt.SigningMethodHS256, secret, jwt.MapClaims{"exp": future}),
```

If the bug was a race, make the test hit the race window on purpose. For
example, send two messages back to back without waiting for the reply in between,
or type before the server has answered. Run it many times before calling it
fixed: `go test -race -count=50 -run TestX ./...` or a shell loop for the browser suite.
