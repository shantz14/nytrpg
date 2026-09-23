// Shared setup for client unit tests. They run in Node against the compiled
// JS in client/static (npm test compiles first), so only code that doesn't
// touch the DOM at import time can be tested here; the browser suite covers
// the rest.
import * as MessagePack from "@msgpack/msgpack";

// net.ts uses the MessagePack global the page loads from a CDN
globalThis.MessagePack = MessagePack;

// Replaces performance.now with a clock the test controls
export function fakeClock(start = 1000) {
    const real = performance.now;
    let now = start;
    performance.now = () => now;
    return {
        set: (t) => { now = t; },
        advance: (ms) => { now += ms; },
        get: () => now,
        restore: () => { performance.now = real; },
    };
}
