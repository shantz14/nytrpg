// End-to-end tests in a real (headless) browser: builds the server, starts it
// on a free port with a throwaway database, and drives the actual UI.
//
//   npm run test:browser
//
// Needs Chromium or Chrome. Set CHROME_PATH if it isn't found automatically.
import puppeteer from "puppeteer-core";
import { decode } from "@msgpack/msgpack";
import { spawn, execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const ROOT = resolve(import.meta.dirname, "../../..");
const VIEW = { width: 1280, height: 800 };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---- test harness ----

const tests = [];
const test = (name, fn) => tests.push({ name, fn });
function assert(ok, msg) {
    if (!ok) throw new Error(msg);
}

// ---- server ----

const tmp = mkdtempSync(join(tmpdir(), "nytrpg-browser-"));
const bin = join(tmp, "server");
let server = null;
let port = 0;

function freePort() {
    return new Promise((res) => {
        const s = createServer().listen(0, () => {
            const p = s.address().port;
            s.close(() => res(p));
        });
    });
}

async function startServer() {
    server = spawn(bin, [], {
        env: { ...process.env, PORT: String(port), JWT_SECRET: "browser-test", DB_PATH: join(tmp, "test.db"), STATIC_DIR: join(ROOT, "client/static") },
        stdio: ["ignore", "ignore", "pipe"],
    });
    server.stderr.on("data", (d) => { if (process.env.VERBOSE) process.stderr.write(d); });
    for (let i = 0; i < 100; i++) {
        try {
            if ((await fetch(base())).ok) return;
        } catch { /* not up yet */ }
        await sleep(100);
    }
    throw new Error("server didn't start");
}

async function stopServer() {
    if (!server) return;
    const exited = new Promise((r) => server.once("exit", r));
    server.kill("SIGTERM");
    await exited;
    server = null;
}

const base = () => `http://localhost:${port}`;

// ---- browser helpers ----

function chromePath() {
    const candidates = [process.env.CHROME_PATH, "/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome",
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"];
    const found = candidates.find((p) => p && existsSync(p));
    if (!found) throw new Error("No Chromium/Chrome found, set CHROME_PATH");
    return found;
}

let browser;
let names = 0;
const errors = [];

// A logged in player in their own browser context. Tracks the player's
// position and every websocket frame, decoded.
async function player() {
    const name = `p${Date.now() % 100000}_${names++}`;
    await fetch(base() + "/signup", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username: name, password: "pw" }) });

    const ctx = await browser.createBrowserContext();
    const page = await ctx.newPage();
    await page.setViewport(VIEW);
    page.on("pageerror", (e) => errors.push(`[${name}] ${e.message}`));
    page.on("console", (m) => { if (m.type() === "error") errors.push(`[${name}] console: ${m.text()}`); });

    const cdp = await page.createCDPSession();
    await cdp.send("Network.enable");
    page.pos = null;
    page.frames = [];
    const onFrame = (dir) => (e) => {
        if (e.response.opcode !== 2) return;
        const [t, d] = decode(Buffer.from(e.response.payloadData, "base64"));
        page.frames.push({ dir, t, d });
        if (dir === "in" && t === 1) page.pos = d.pos;  // welcome
        if (dir === "in" && t === 6) page.pos = d;      // correction
        if (dir === "out" && t === 1) page.pos = d;     // our move
    };
    cdp.on("Network.webSocketFrameReceived", onFrame("in"));
    cdp.on("Network.webSocketFrameSent", onFrame("out"));

    await page.evaluateOnNewDocument(recordCharacterDraws);
    await page.goto(base(), { waitUntil: "networkidle0" });
    await page.type("#uname", name);
    await page.type("#psw", "pw");
    await page.click("#submitLogin");
    await page.waitForFunction(() => !document.getElementById("loginPopup"), { timeout: 5000 });
    await waitFor(() => page.pos, "welcome");
    page.name = name;
    return page;
}

async function waitFor(fn, what, timeout = 5000) {
    const end = Date.now() + timeout;
    while (Date.now() < end) {
        const v = await fn();
        if (v) return v;
        await sleep(50);
    }
    throw new Error(`timed out waiting for ${what}`);
}

// Screen point of a world point: the camera keeps the player in the middle
const toScreen = (page, x, y) => [x - page.pos.x + VIEW.width / 2, y - page.pos.y + VIEW.height / 2];

async function hold(page, key, ms) {
    await page.keyboard.down(key);
    await sleep(ms);
    await page.keyboard.up(key);
}

const received = (page, type) => page.frames.filter((f) => f.dir === "in" && f.t === type);
const visible = (page, sel) => page.$eval(sel, (e) => getComputedStyle(e).display !== "none").catch(() => false);

// Runs in the page before any game code: records every character sprite drawn
// on the canvas (which file, which frame, mirrored or not, where) into
// window.__draws, so tests can check animation without comparing pixels.
function recordCharacterDraws() {
    window.__draws = [];
    const drawImage = CanvasRenderingContext2D.prototype.drawImage;
    CanvasRenderingContext2D.prototype.drawImage = function (img, ...args) {
        const file = (img.src || "").split("/").pop();
        if (file === "Skoobyuboo.png" || file === "player-walk.png") {
            const m = this.getTransform();
            const dpr = window.devicePixelRatio || 1;
            const dx = args.length === 8 ? args[4] : args[0];
            window.__draws.push({
                file,
                sx: args.length === 8 ? args[0] : null,
                mirrored: m.a < 0,
                // Left edge on screen in CSS px: a mirrored draw is translated to x + width
                x: m.e / dpr + dx - (m.a < 0 ? 64 : 0),
                t: performance.now(),
            });
            if (window.__draws.length > 5000) window.__draws.splice(0, 2500);
        }
        return drawImage.call(this, img, ...args);
    };
}

// Character draws since page time t. self: our own player (drawn mid-screen) or everyone else.
async function draws(page, t, self) {
    const all = await page.evaluate((t) => window.__draws.filter((d) => d.t >= t), t);
    return all.filter((d) => (d.x === VIEW.width / 2) === self);
}
const pageNow = (page) => page.evaluate(() => performance.now());

// World positions from internal/game/maps/town.json
const BOARD = { x: 750 + 64, y: 500 + 64 };
const LEADERBOARD = { x: 200 + 64, y: 200 + 64 };
const SPEED = 450;

// ---- tests ----

test("two players see each other move, and chat", async () => {
    const a = await player();
    const b = await player();
    await hold(b, "d", 500);
    await waitFor(() => received(a, 5).some((f) => f.d.move?.length), "a to receive b's moves");

    await a.keyboard.press("/");
    await a.keyboard.type("hello");
    await a.keyboard.press("Enter");
    await waitFor(() => received(b, 4).some((f) => f.d.msg === "hello"), "b to hear the chat");
    await a.browserContext().close();
    await b.browserContext().close();
});

test("wordle: play, reload, guesses come back; popups block the world", async () => {
    const p = await player();
    await p.mouse.click(...toScreen(p, BOARD.x, BOARD.y));
    await p.waitForSelector("#letter-0-0", { timeout: 3000 });
    assert((await p.$$(".wordContainer")).length === 5, "5 rows");

    // Typed and submitted immediately, usually before the server's reply to opening
    // arrives. That used to reset the row counter and lose the guess.
    await p.keyboard.type("crane");
    await p.keyboard.press("Enter");
    // Wait on the page, not the frame: the frame shows up before the page handles it
    await waitFor(() => p.$$eval("#wordContainer0 input", (els) => els.every((e) => e.style.backgroundColor)), "first row colored");
    assert(/^\d+:\d\d$/.test(await p.$eval("#timer", (e) => e.textContent)), "timer running");

    // Clicking the world under an open popup does nothing
    await p.mouse.click(...toScreen(p, BOARD.x, BOARD.y));
    await sleep(200);
    assert((await p.$$("#wordlePopup")).length === 1, "second wordle opened through the popup");

    p.pos = null;
    await p.reload({ waitUntil: "networkidle0" });
    await waitFor(() => p.pos, "welcome after reload");
    await p.mouse.click(...toScreen(p, BOARD.x, BOARD.y));
    await p.waitForSelector("#letter-0-0", { timeout: 3000 });
    await waitFor(async () => (await p.$$eval("#wordContainer0 input", (els) => els.map((e) => e.value).join(""))) === "CRANE", "guess restored");

    await p.keyboard.press("Escape");
    await sleep(100);
    assert((await p.$$("#wordlePopup")).length === 0, "Escape closes the popup");
    await p.browserContext().close();
});

test("walking is never corrected; far things say walk closer; leaderboard opens", async () => {
    const p = await player();
    await hold(p, "a", Math.max(0, (p.pos.x - LEADERBOARD.x) / SPEED * 1000 - 300));
    await hold(p, "w", Math.max(0, (p.pos.y - LEADERBOARD.y) / SPEED * 1000 - 300));
    await sleep(300);
    assert(received(p, 6).length === 0, "the server corrected normal walking");

    // The board is out of range from here
    await p.mouse.click(...toScreen(p, BOARD.x, BOARD.y));
    await waitFor(() => visible(p, "#toast"), "walk closer hint");
    assert(!(await p.$("#wordlePopup")), "wordle opened from too far away");

    await p.mouse.click(...toScreen(p, LEADERBOARD.x, LEADERBOARD.y));
    await p.waitForSelector("#leaderboardPopup", { timeout: 3000 });
    await waitFor(async () => (await p.$$("#lbBody tr")).length > 0, "leaderboard rows");
    assert(await p.$eval("#nextDay", (b) => b.disabled), "next day is disabled on today");
    await p.browserContext().close();
});

test("character walks with the sprite sheet, flips left, and faces the camera when idle", async () => {
    const watcher = await player();
    const p = await player();
    await sleep(300);

    let t = await pageNow(p);
    await sleep(200);
    let own = await draws(p, t, true);
    assert(own.length && own.every((d) => d.file === "Skoobyuboo.png"), `idle should face the camera: ${JSON.stringify(own.slice(-3))}`);

    // Walking right: the sheet, not mirrored, cycling through frames
    t = await pageNow(p);
    await hold(p, "d", 700);
    own = (await draws(p, t, true)).filter((d) => d.file === "player-walk.png");
    assert(own.length > 10, `walking should draw the sheet, drew ${own.length}`);
    assert(own.every((d) => !d.mirrored), "walking right is drawn mirrored");
    const frames = new Set(own.map((d) => d.sx));
    assert(frames.size >= 4, `frames should cycle, saw ${[...frames]}`);

    // Walking left: mirrored
    t = await pageNow(p);
    const tWatch = await pageNow(watcher);
    await hold(p, "a", 700);
    own = (await draws(p, t, true)).filter((d) => d.file === "player-walk.png");
    assert(own.length > 10 && own.every((d) => d.mirrored), "walking left should be mirrored");

    // The other player sees it too, mirrored while walking left
    const seen = await draws(watcher, tWatch, false);
    assert(seen.some((d) => d.file === "player-walk.png" && d.mirrored), `watcher should see p walk left: ${JSON.stringify(seen.slice(-3))}`);

    // Stopped: back to facing the camera, for both of them
    await sleep(500);
    t = await pageNow(p);
    const tw = await pageNow(watcher);
    await sleep(200);
    own = await draws(p, t, true);
    assert(own.length && own.every((d) => d.file === "Skoobyuboo.png"), "idle again after stopping");
    const seenIdle = await draws(watcher, tw, false);
    assert(seenIdle.length && seenIdle.every((d) => d.file === "Skoobyuboo.png"), "watcher should see p idle");

    await watcher.browserContext().close();
    await p.browserContext().close();
});

test("reconnects after the server restarts", async () => {
    const p = await player();
    const welcomes = () => received(p, 1).length;
    assert(welcomes() === 1, "one welcome");

    await stopServer();
    await waitFor(() => visible(p, "#reconnecting"), "reconnecting banner");
    await startServer();
    await waitFor(async () => !(await visible(p, "#reconnecting")), "banner to clear", 15000);
    await waitFor(() => welcomes() === 2, "a new welcome");

    // Still playable
    const moves = p.frames.filter((f) => f.dir === "out" && f.t === 1).length;
    await hold(p, "d", 300);
    await waitFor(() => p.frames.filter((f) => f.dir === "out" && f.t === 1).length > moves, "moves after reconnecting");
    await p.browserContext().close();
});

// ---- run ----

let failed = 0;
try {
    execFileSync("go", ["build", "-o", bin, "./cmd/server"], { cwd: ROOT, stdio: "inherit" });
    port = await freePort();
    await startServer();
    browser = await puppeteer.launch({ executablePath: chromePath(), headless: true, args: ["--no-sandbox"] });

    for (const { name, fn } of tests) {
        const before = errors.length;
        try {
            await fn();
            if (errors.length > before) throw new Error("page errors:\n  " + errors.slice(before).join("\n  "));
            console.log(`✔ ${name}`);
        } catch (e) {
            failed++;
            console.log(`✖ ${name}\n  ${e.message}`);
        }
    }
} finally {
    await browser?.close();
    await stopServer();
    rmSync(tmp, { recursive: true, force: true });
}
console.log(`\n${tests.length - failed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
