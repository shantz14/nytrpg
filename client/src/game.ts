import { DisplayDriver } from "./display-driver.js";
import { InputDriver } from "./input-driver.js";
import { Clickable, GameState, PlayerState} from "./game-objects.js";
import { Vector2D } from "./vector2D.js";
import { Wordle } from "./wordle.js";
import { Leaderboard } from "./leaderboard.js";
import { ClientUpdate, ClientUpdatePos, ClientUpdateType, ServerUpdate, ServerUpdatePos, ServerWordleResponse, UpdateState, WordleReq, WordleResponse, Chat, ServerChat, ClientChat } from "./messages.js";
import { UserData, logout } from "./login.js";

declare const MessagePack: typeof import("@msgpack/msgpack");
const encode = MessagePack.encode;
const decode = MessagePack.decode;

const SERVER_URL = "/ws";

export class Game {
    sock: WebSocket;
    displayDriver: DisplayDriver;
    inputDriver: InputDriver;
    state: GameState;
    wordle: Wordle | null;
    userData: UserData;

    constructor(ctx: CanvasRenderingContext2D, userData: UserData) {
        const canvas = ctx.canvas;

        this.sock = new WebSocket(SERVER_URL + `?id=${userData.id}`);
        this.state = new GameState();
        this.inputDriver = new InputDriver(canvas, this.state);
        const middle = this.findMiddle();
        this.displayDriver = new DisplayDriver(ctx, this.state, userData, middle);
        this.wordle = null;
        this.userData = userData;
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
        this.sock.onmessage = async (e) => {
            let rawMsg: ArrayBuffer;

            if (e.data instanceof Blob) {
                rawMsg = await e.data.arrayBuffer();
            } else {
                rawMsg = e.data;
            }
            const msg = decode(new Uint8Array(rawMsg)) as ServerUpdate;

            if (msg.updateType == ServerUpdatePos) {
                const update = decode(msg.data) as UpdateState;
                this.updatePos(update);
            } else if (msg.updateType == ServerWordleResponse) {
                const update = decode(msg.data) as WordleResponse;
                if (this.wordle) {
                    this.wordle.handleResponse(update);
                } else {
                    console.log("Guys there's no wordle why are we sending wordle updates.");
                }
            } else if (msg.updateType == ServerChat) {
                const update = decode(msg.data) as Chat;
                this.displayDriver.updateChat(update);
            } else {
                console.log("Wacky msg from server.");
            }

        }
    }

    private updatePos(update: UpdateState) {
        this.unregister(update.unregister);

        for (const id in update.players) {
            const newState = update.players[id];
            if (newState.me) {
                // This is the players own data

            } else {
                // Another players data
                this.state.otherChars[Number(id)] = structuredClone(newState);
            }

        }

    }

    private unregister(id: number) {
        delete this.state.otherChars[id];
        delete this.displayDriver.state.otherChars[id];

        this.displayDriver.images.delete(String(id));
    }
    
    private sendPlayerState() {
        const data = new PlayerState();
        data.pos.x = this.state.charVec.x + this.displayDriver.middle.x;
        data.pos.y = this.state.charVec.y + this.displayDriver.middle.y;
        data.id = this.userData.id;

        this.send(ClientUpdatePos, data);
    }

    public send(type: ClientUpdateType, data: PlayerState | WordleReq | Chat) {
        const envelope: ClientUpdate = {
            updateType: type,
            data: encode(data)
        };

        const encoded: Uint8Array = encode(envelope);

        if (this.sock.readyState === WebSocket.OPEN) {
            this.sock.send(encoded);
        }
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
            const chat: Chat = {
                id: this.userData.id,
                msg: chatbox.value
            }
            this.send(ClientChat, chat);
        });
    }

}
