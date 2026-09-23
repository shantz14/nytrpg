import { test } from "node:test";
import assert from "node:assert/strict";
import { fakeClock } from "./setup.mjs";
import { RemoteEntity, Clickable, Rect, INTERP_DELAY_MS } from "../static/game-objects.js";
import { Vector2D } from "../static/vector2D.js";

test("remote entities are drawn INTERP_DELAY_MS in the past, interpolated", () => {
    const clock = fakeClock(1000);
    try {
        const e = new RemoteEntity(1, "b", "s.png", { x: 0, y: 0 });
        clock.set(1050); e.addSample(100, 0);
        clock.set(1100); e.addSample(200, 0);

        e.interpolate(1025 + INTERP_DELAY_MS);        // halfway between the samples at 1000 and 1050
        assert.deepEqual(e.pos, { x: 50, y: 0 });
        e.interpolate(1075 + INTERP_DELAY_MS);        // halfway between 1050 and 1100
        assert.deepEqual(e.pos, { x: 150, y: 0 });
        e.interpolate(1100 + INTERP_DELAY_MS);        // exactly the last sample
        assert.deepEqual(e.pos, { x: 200, y: 0 });
        e.interpolate(5000);                          // no newer data: stay put, don't extrapolate
        assert.deepEqual(e.pos, { x: 200, y: 0 });
    } finally {
        clock.restore();
    }
});

test("an entity starting to move after standing still glides instead of jumping", () => {
    const clock = fakeClock(1000);
    try {
        const e = new RemoteEntity(1, "b", "s.png", { x: 0, y: 0 });
        clock.set(10000); e.addSample(20, 0);           // first move after 9s idle
        e.interpolate(10000 + INTERP_DELAY_MS - 25);    // half a tick before the move lands
        assert.ok(e.pos.x > 0 && e.pos.x < 20, `should be partway, is at ${e.pos.x}`);
    } finally {
        clock.restore();
    }
});

test("the sample buffer stays small", () => {
    const clock = fakeClock(0);
    try {
        const e = new RemoteEntity(1, "b", "s.png", { x: 0, y: 0 });
        for (let i = 1; i <= 1000; i++) { clock.set(i * 50); e.addSample(i, 0); }
        assert.ok(e["samples"].length <= 5, `buffer grew to ${e["samples"].length}`);
    } finally {
        clock.restore();
    }
});

test("Clickable.inRange matches the server's distance to the rectangle's edge", () => {
    const board = new Clickable("wordle", new Vector2D(100, 100), 50, 50, () => {}, 10);
    assert.ok(board.inRange(new Vector2D(120, 120)), "inside");
    assert.ok(board.inRange(new Vector2D(90, 120)), "10px left");
    assert.ok(board.inRange(new Vector2D(160, 120)), "10px right of x+w");
    assert.ok(!board.inRange(new Vector2D(161, 120)), "11px right");
    assert.ok(!board.inRange(new Vector2D(158, 158)), "corner beyond range");
    const anywhere = new Clickable("sign", new Vector2D(0, 0), 10, 10, () => {});
    assert.ok(anywhere.inRange(new Vector2D(99999, 99999)), "range 0 is anywhere");
});

test("Rect follows the camera", () => {
    const r = new Rect(new Vector2D(100, 100), 20, 20);
    r.adjust(new Vector2D(50, 50));
    assert.ok(r.inRect(new Vector2D(60, 60)));
    assert.ok(!r.inRect(new Vector2D(100, 100)));
});
