import { DisplayDriver } from "./display-driver.js";
import { InputDriver } from "./input-driver.js";
import { Clickable, GameState } from "./game-objects.js";
import { Vector2D } from "./vector2D.js";
import { Wordle } from "./wordle.js";
import { Leaderboard } from "./leaderboard.js";
import { ChatMsg, ChatReq, ClientChat, ClientMove, ClientMsg, PlayerSnap, ServerChat, ServerSnapshot, ServerWelcome, ServerWordleResult, ServerWordleResume, Snapshot, Vec, Welcome, WordleRes, WordleResume } from "./protocol.gen.js";
import { Connection } from "./net.js";
import { UserData, logout } from "./login.js";

const SERVER_URL = "/ws";

export class Game {
    conn: Connection;
    displayDriver: DisplayDriver;
    inputDriver: InputDriver;
    state: GameState;
    wordle: Wordle | null;
    userData: UserData;
    // Last position sent, so we only send when we move
    lastSent: Vec | null;

    constructor(ctx: CanvasRenderingContext2D, userData: UserData) {
        const canvas = ctx.canvas;

        this.conn = new Connection(SERVER_URL + `?token=${encodeURIComponent(userData.jwt)}`);
        this.state = new GameState();
        this.inputDriver = new InputDriver(canvas, this.state);
        const middle = this.findMiddle();
        this.displayDriver = new DisplayDriver(ctx, this.state, userData, middle);
        this.wordle = null;
        this.userData = userData;
        this.lastSent = null;
    }

    public run() {
        this.handleMsgs();
        this.handleChats();
        this.createUI();
        this.createMap();
        setInterval(() => {
            this.update();
        }, 34);
    }

    private handleMsgs() {
        this.conn.on<Welcome>(ServerWelcome, (welcome) => {
            console.log("Connected as", welcome.username);
        });
        this.conn.on<Snapshot>(ServerSnapshot, (snap) => this.updatePos(snap));
        this.conn.on<ChatMsg>(ServerChat, (chat) => this.displayDriver.updateChat(chat));
        this.conn.on<WordleRes>(ServerWordleResult, (res) => this.wordle?.handleResponse(res));
        this.conn.on<WordleResume>(ServerWordleResume, (resume) => this.wordle?.handleResume(resume));
    }

    // Each update is a full snapshot, so anyone not in it has left
    private updatePos(snap: Snapshot) {
        const others: {[key: number]: PlayerSnap} = {};

        for (const player of snap.players) {
            if (player.id != this.userData.id) {
                others[player.id] = player;
            }
        }

        for (const id in this.state.otherChars) {
            if (!(id in others)) {
                this.displayDriver.removePlayer(Number(id));
            }
        }

        this.state.otherChars = others;
    }

    private sendPlayerState() {
        const pos: Vec = {
            x: Math.round(this.state.charVec.x + this.displayDriver.middle.x),
            y: Math.round(this.state.charVec.y + this.displayDriver.middle.y),
        };
        if (this.lastSent && this.lastSent.x == pos.x && this.lastSent.y == pos.y) {
            return;
        }
        if (this.conn.send(ClientMove, pos)) {
            this.lastSent = pos;
        }
    }

    public send(type: ClientMsg, data: unknown): boolean {
        return this.conn.send(type, data);
    }

    private update() {
        this.loadOtherCharSprites();
        this.sendPlayerState();

        this.move();
        this.displayDriver.draw();

    }

    private createUI() {
        this.createUIClickable("logout", new Vector2D(750, 200), 128, 128, "logout.png", logout);
        this.createUIClickable("leaderboard", new Vector2D(200, 200), 128, 128, "logout.png", () => {
            const lb = new Leaderboard(this.userData, this.inputDriver);
            lb.run();
        });
    }

    private createMap() {
        /*this.createClickable("guide", new Vector2D(500, 500), 64, 64, "Skoobyuboo.png", function() {
            console.log("CLICKED");
        });*/

        this.createClickable("playWordle", new Vector2D(750, 500), 128, 128, "GameBoard.png", () => {
            this.wordle = new Wordle(this);
            this.wordle.run();
        });

    }

    private createClickable(name: string, pos: Vector2D, height: number, width: number, asset: string, action: Function) {
        const guide = new Clickable(name, pos, height, width, action);
        this.state.clickables[name] = guide;
        if (!this.displayDriver.images.has(name)) {
            this.displayDriver.loadImage(name, asset);
        }
    }

    private createUIClickable(name: string, pos: Vector2D, height: number, width: number, asset: string, action: Function) {
        const guide = new Clickable(name, pos, height, width, action);
        this.state.clickables[name] = guide;
        if (!this.displayDriver.images.has(name)) {
            this.displayDriver.loadImage(name, asset);
        }
    }

    private move() {
        if (!this.inputDriver.isGameFocused()) {
            return;
        }

        let movement = new Vector2D(0, 0); 

        if (this.inputDriver.keysPressed.has("w") && !this.inputDriver.keysPressed.has("s")) {
            movement.y = -15;
        }
        else if (this.inputDriver.keysPressed.has("s") && !this.inputDriver.keysPressed.has("w")) {
            movement.y = 15;
        }

        if (this.inputDriver.keysPressed.has("a") && !this.inputDriver.keysPressed.has("d")) {
            movement.x = -15;
        }
        else if (this.inputDriver.keysPressed.has("d") && !this.inputDriver.keysPressed.has("a")) {
            movement.x = 15;
        }

        this.state.charVec.add(movement);
    }

    private loadOtherCharSprites() {
        for (const id in this.state.otherChars) {
            // Other player sprites stored unter their integer id as a string
            if (!this.displayDriver.images.has(String(id))) {
                this.displayDriver.loadImage(String(id), "Skoobyuboo.png");
            }
        }
    }

    private findMiddle(): Vector2D {
        const x = window.innerWidth / 2;
        const y = window.innerHeight / 2;
        return new Vector2D(x, y);
    }

    private handleChats() {
        const chatbox = document.getElementById("chatbox") as HTMLInputElement;
        chatbox.addEventListener("sendChat", () => {
            const chat: ChatReq = { msg: chatbox.value };
            this.send(ClientChat, chat);
        });
    }

}
