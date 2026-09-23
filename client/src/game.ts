import { DisplayDriver } from "./display-driver.js";
import { InputDriver } from "./input-driver.js";
import { Clickable, GameState, RemoteEntity } from "./game-objects.js";
import { Vector2D } from "./vector2D.js";
import { Wordle } from "./wordle.js";
import { Leaderboard } from "./leaderboard.js";
import { ChatMsg, ChatReq, ClientChat, ClientMove, ClientMsg, ServerChat, ServerCorrection, ServerWelcome, ServerWorld, ServerWordleResult, ServerWordleResume, Vec, Welcome, WorldMap, WorldUpdate, WordleRes, WordleResume } from "./protocol.gen.js";
import { Connection } from "./net.js";
import { UserData, logout } from "./login.js";

const SERVER_URL = "/ws";
// Position updates per second, matches the server tick rate
const SEND_RATE = 20;

export class Game {
    conn: Connection;
    displayDriver: DisplayDriver;
    inputDriver: InputDriver;
    state: GameState;
    wordle: Wordle | null;
    userData: UserData;
    // Last position sent, so we only send when we move
    lastSent: Vec | null;
    lastSentAt: number;
    lastFrame: number;
    // px/s, from the server. 0 until the welcome arrives, so we can't move before then.
    moveSpeed: number;

    constructor(ctx: CanvasRenderingContext2D, userData: UserData) {
        const canvas = ctx.canvas;

        this.state = new GameState();
        this.inputDriver = new InputDriver(canvas, this.state);
        this.displayDriver = new DisplayDriver(ctx, this.state);
        this.wordle = null;
        this.userData = userData;
        this.lastSent = null;
        this.lastSentAt = 0;
        this.lastFrame = performance.now();
        this.moveSpeed = 0;
        this.conn = new Connection(SERVER_URL + `?token=${encodeURIComponent(userData.jwt)}`);
    }

    public run() {
        this.handleMsgs();
        this.handleChats();
        this.conn.onDisconnect = () => this.showReconnecting(true);
        this.conn.onReconnect = () => this.showReconnecting(false);
        this.conn.onReplaced = () => {
            const banner = document.getElementById("reconnecting")!;
            banner.textContent = "You logged in somewhere else. Reload to play here.";
            banner.style.display = "flex";
        };
        requestAnimationFrame((t) => this.frame(t));
    }

    private frame(now: number) {
        const dt = Math.min((now - this.lastFrame) / 1000, 0.1);
        this.lastFrame = now;

        this.move(dt);
        this.sendPlayerState(now);
        for (const id in this.state.otherChars) {
            this.state.otherChars[id].interpolate(now);
        }
        this.displayDriver.updateCamera();
        this.displayDriver.draw();

        requestAnimationFrame((t) => this.frame(t));
    }

    private handleMsgs() {
        this.conn.on<Welcome>(ServerWelcome, (welcome) => this.welcome(welcome));
        this.conn.on<WorldUpdate>(ServerWorld, (upd) => this.applyWorldUpdate(upd));
        this.conn.on<Vec>(ServerCorrection, (pos) => this.setPosition(pos));
        this.conn.on<ChatMsg>(ServerChat, (chat) => this.displayDriver.updateChat(chat));
        this.conn.on<WordleRes>(ServerWordleResult, (res) => this.wordle?.handleResponse(res));
        this.conn.on<WordleResume>(ServerWordleResume, (resume) => this.wordle?.handleResume(resume));
    }

    // Sent on every connect, including reconnects: start from a clean slate
    private welcome(welcome: Welcome) {
        this.state.selfId = welcome.entityId;
        this.state.selfName = welcome.username;
        this.moveSpeed = welcome.moveSpeed;
        for (const id in this.state.otherChars) {
            this.displayDriver.removePlayer(Number(id));
        }
        this.state.otherChars = {};
        this.setPosition(welcome.pos);
        this.createMap(welcome.map);
    }

    // Moves us to a world position, e.g. when the server corrects us
    private setPosition(pos: Vec) {
        this.state.selfPos.set(pos.x, pos.y);
        this.lastSent = pos;
    }

    private applyWorldUpdate(upd: WorldUpdate) {
        for (const e of upd.spawn ?? []) {
            this.state.otherChars[e.id] = new RemoteEntity(e.id, e.name, e.sprite, e.pos);
        }
        for (const [id, x, y] of upd.move ?? []) {
            this.state.otherChars[id]?.addSample(x, y);
        }
        for (const id of upd.despawn ?? []) {
            delete this.state.otherChars[id];
            this.displayDriver.removePlayer(id);
        }
    }

    private sendPlayerState(now: number) {
        // No faster than the server ticks
        if (this.moveSpeed == 0 || now - this.lastSentAt < 1000 / SEND_RATE) {
            return;
        }
        const pos: Vec = {
            x: Math.round(this.state.selfPos.x),
            y: Math.round(this.state.selfPos.y),
        };
        if (this.lastSent && this.lastSent.x == pos.x && this.lastSent.y == pos.y) {
            return;
        }
        if (this.conn.send(ClientMove, pos)) {
            this.lastSent = pos;
            this.lastSentAt = now;
        }
    }

    public send(type: ClientMsg, data: unknown): boolean {
        return this.conn.send(type, data);
    }

    // What clicking each kind of interactable does
    private actions: {[action: string]: () => void} = {
        wordle: () => {
            const wordle = new Wordle(this);
            if (wordle.run()) {
                this.wordle = wordle;
            }
        },
        leaderboard: () => new Leaderboard(this.userData, this.inputDriver).run(),
        logout: () => {
            this.conn.close();
            logout();
        },
    };

    private createMap(map: WorldMap) {
        this.displayDriver.loadImage("bg", map.background);
        this.state.clickables = {};
        for (const it of map.interactables) {
            const action = this.actions[it.action];
            if (!action) {
                console.warn("Unknown interactable action", it.action);
                continue;
            }
            this.state.clickables[it.id] = new Clickable(it.id, new Vector2D(it.pos.x, it.pos.y), it.h, it.w, action);
            this.displayDriver.loadImage(it.id, it.sprite);
        }
    }

    // Moves at moveSpeed px/s in the pressed direction, diagonals aren't faster
    private move(dt: number) {
        if (!this.inputDriver.isGameFocused() || this.moveSpeed == 0) {
            return;
        }
        const keys = this.inputDriver.keysPressed;
        let x = 0, y = 0;
        if (keys.has("w")) y -= 1;
        if (keys.has("s")) y += 1;
        if (keys.has("a")) x -= 1;
        if (keys.has("d")) x += 1;
        if (x == 0 && y == 0) {
            return;
        }
        const len = Math.hypot(x, y);
        const step = this.moveSpeed * dt;
        this.state.selfPos.add(new Vector2D(x / len * step, y / len * step));
    }

    private showReconnecting(show: boolean) {
        document.getElementById("reconnecting")!.style.display = show ? "flex" : "none";
    }

    private handleChats() {
        const chatbox = document.getElementById("chatbox") as HTMLInputElement;
        chatbox.addEventListener("sendChat", () => {
            const chat: ChatReq = { msg: chatbox.value };
            this.send(ClientChat, chat);
        });
    }

}
