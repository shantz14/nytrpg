import { test, mock } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { LatestThrottle } from "../static/throttle.js";
import { RemoteEntity, pickEntity } from "../static/game-objects.js";

// A throttle on a clock the test moves, with setTimeout mocked to match
function throttled(intervalMs = 50) {
    let now = 1000;
    const sent = [];
    const t = new LatestThrottle((v) => sent.push(v), intervalMs, () => now);
    const advance = (ms) => {
        now += ms;
        mock.timers.tick(ms);
    };
    return { t, sent, advance };
}

test("typing throttle: the first change goes out at once, a burst ends on the latest", () => {
    mock.timers.enable({ apis: ["setTimeout"] });
    try {
        const { t, sent, advance } = throttled();
        t.push(1);
        assert.deepEqual(sent, [1], "first change is instant");

        // Typing fast: 2, 3, 4 within the interval collapse to 4
        advance(10); t.push(2);
        advance(10); t.push(3);
        advance(10); t.push(4);
        assert.deepEqual(sent, [1]);
        advance(20);
        assert.deepEqual(sent, [1, 4], "sent once the interval is up, with the newest count");

        // After a pause the next change is instant again
        advance(200);
        t.push(0);
        assert.deepEqual(sent, [1, 4, 0]);
    } finally {
        mock.timers.reset();
    }
});

test("typing throttle: never faster than the interval, skips repeats, cancel drops pending", () => {
    mock.timers.enable({ apis: ["setTimeout"] });
    try {
        const { t, sent, advance } = throttled();
        // 5 letters a millisecond apart for a second, as fast as anyone could type
        for (let i = 0; i < 1000; i++) {
            t.push(i % 6);
            advance(1);
        }
        assert.ok(sent.length <= 1000 / 50 + 1, `sent ${sent.length} in 1s`);

        advance(100);
        const n = sent.length;
        t.push(sent[n - 1]);
        advance(100);
        assert.equal(sent.length, n, "the value already sent isn't sent again");

        // Held back, then typed back to what was last sent: nothing to send
        t.push(sent[n - 1] === 1 ? 2 : 1);
        t.push(sent[n - 1] === 1 ? 3 : 4);
        advance(1);
        const m = sent.length;
        t.push(9);
        t.cancel();
        advance(100);
        assert.equal(sent.length, m, "cancelled updates never go out");
    } finally {
        mock.timers.reset();
    }
});

test("pickEntity finds the player under the cursor, the one drawn last when they overlap", () => {
    const a = new RemoteEntity(1, "a", "s.png", { x: 0, y: 0 });
    const b = new RemoteEntity(2, "b", "s.png", { x: 0, y: 0 });
    const off = new RemoteEntity(3, "c", "s.png", { x: 0, y: 0 });
    a.hitbox = { x: 100, y: 100, w: 48, h: 64 };
    b.hitbox = { x: 130, y: 100, w: 48, h: 64 };
    off.hitbox = null; // off screen, not drawn

    assert.equal(pickEntity([a, b, off], 110, 120), a);
    assert.equal(pickEntity([a, b, off], 140, 120), b, "overlap goes to the one on top");
    assert.equal(pickEntity([a, b, off], 170, 163), b);
    assert.equal(pickEntity([a, b, off], 99, 120), null);
    assert.equal(pickEntity([a, b, off], 110, 165), null);
    assert.equal(pickEntity([], 110, 120), null);
});
