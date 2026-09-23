import { test, beforeEach, afterEach, mock } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { Connection } from "../static/net.js";

const { encode, decode } = globalThis.MessagePack;

// A WebSocket the test drives by hand
class FakeSocket {
    static OPEN = 1;
    static all = [];
    constructor(url) {
        this.url = url;
        this.readyState = 0;
        this.sent = [];
        FakeSocket.all.push(this);
    }
    send(data) { this.sent.push(decode(data)); }
    close() { this.readyState = 3; }
    // test controls
    open() { this.readyState = FakeSocket.OPEN; this.onopen?.(); }
    receive(type, payload) {
        // encode() returns a view into a bigger buffer; a real socket delivers just the message
        const bytes = encode([type, payload]);
        this.onmessage?.({ data: bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) });
    }
    drop(code = 1006) { this.readyState = 3; this.onclose?.({ code }); }
}

let realWebSocket;
beforeEach(() => {
    realWebSocket = globalThis.WebSocket;
    globalThis.WebSocket = FakeSocket;
    FakeSocket.all = [];
    mock.timers.enable({ apis: ["setTimeout"] });
});
afterEach(() => {
    mock.timers.reset();
    globalThis.WebSocket = realWebSocket;
});

const latest = () => FakeSocket.all[FakeSocket.all.length - 1];

test("messages go to the handler for their type", () => {
    const conn = new Connection("/ws");
    const got = [];
    conn.on(5, (d) => got.push(d));
    latest().open();
    latest().receive(5, { hello: 1 });
    latest().receive(9, {}); // no handler: ignored
    assert.deepEqual(got, [{ hello: 1 }]);
    assert.throws(() => conn.on(5, () => {}), /already registered/);
});

test("send is [type, payload] and reports whether it went out", () => {
    const conn = new Connection("/ws");
    assert.equal(conn.send(1, { x: 1 }), false, "not open yet");
    latest().open();
    assert.equal(conn.send(1, { x: 1 }), true);
    assert.deepEqual(latest().sent, [[1, { x: 1 }]]);
});

test("reconnects with backoff and reports the outage once", () => {
    const conn = new Connection("/ws");
    let downs = 0, ups = 0;
    conn.onDisconnect = () => downs++;
    conn.onReconnect = () => ups++;
    latest().open();

    latest().drop();
    assert.equal(downs, 1);
    mock.timers.tick(500);
    assert.equal(FakeSocket.all.length, 2, "first retry after 500ms");
    latest().drop();                        // retry fails
    mock.timers.tick(999);
    assert.equal(FakeSocket.all.length, 2, "backoff doubled");
    mock.timers.tick(1);
    assert.equal(FakeSocket.all.length, 3);
    assert.equal(downs, 1, "one outage, one report");

    const handled = [];
    conn.on(1, (d) => handled.push(d));
    latest().open();
    assert.equal(ups, 1);
    latest().receive(1, "welcome");         // handlers survive reconnects
    assert.deepEqual(handled, ["welcome"]);
});

test("closed with 4001 (logged in elsewhere): stops, doesn't fight the other tab", () => {
    const conn = new Connection("/ws");
    let replaced = 0;
    conn.onReplaced = () => replaced++;
    latest().open();
    latest().drop(4001);
    mock.timers.tick(60000);
    assert.equal(replaced, 1);
    assert.equal(FakeSocket.all.length, 1, "no reconnect");
});

test("close() stops reconnecting", () => {
    const conn = new Connection("/ws");
    latest().open();
    conn.close();
    latest().drop();
    mock.timers.tick(60000);
    assert.equal(FakeSocket.all.length, 1);
});
