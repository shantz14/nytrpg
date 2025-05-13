import { DisplayDriver } from "./display-driver.js";
import { InputDriver } from "./input-driver.js";
import { Clickable, GameState, PlayerState, UpdateState } from "./game-objects.js";
import { Vector2D } from "./vector2D.js";
import { Wordle } from "./wordle.js";

declare const MessagePack: typeof import("@msgpack/msgpack");
const encode = MessagePack.encode;
const decode = MessagePack.decode;

const SERVER_URL = "http://localhost:8080/ws";

export class Game {
    sock: WebSocket;
    displayDriver: DisplayDriver;
    inputDriver: InputDriver;
    state: GameState;

    constructor(ctx: CanvasRenderingContext2D) {
        const canvas = ctx.canvas;

        this.sock = new WebSocket(SERVER_URL);
        this.state = new GameState();
        this.inputDriver = new InputDriver(canvas, this.state);
        this.displayDriver = new DisplayDriver(ctx, this.state);
    }

    public run() {
        this.handleMsgs();
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
            const msg = decode(new Uint8Array(rawMsg)) as UpdateState;

            this.unregister(msg.unregister);

            for (const id in msg.players) {
                const newState = msg.players[id];
                if (newState.me) {
                    // This is the players own data

                } else {
                    // Another players data
                    this.state.otherChars[Number(id)] = structuredClone(newState);
                }

            }
        }
    }

    private unregister(id: number) {
        delete this.state.otherChars[id];
        delete this.displayDriver.state.otherChars[id];

        this.displayDriver.images.delete(String(id));
    }

    private send() {
        const msg = new PlayerState();
        msg.pos.x = this.state.charVec.x;
        msg.pos.y = this.state.charVec.y;

        const encoded: Uint8Array = encode(msg);

        if (this.sock.readyState === WebSocket.OPEN) {
            this.sock.send(encoded);
        }
    }

    private update() {
        this.loadOtherCharSprites();
        this.send();

        this.move();
        this.displayDriver.draw();

    }

    private createMap() {
        /*this.createClickable("guide", new Vector2D(500, 500), 64, 64, "Skoobyuboo.png", function() {
            console.log("CLICKED");
        });*/

        this.createClickable("playWordle", new Vector2D(1250, 500), 128, 128, "GameBoard.png", function() {
            const wordle = new Wordle();
            wordle.run();
        });

    }

    private createClickable(name: string, pos: Vector2D, height: number, width: number, asset: string, action: Function) {
        const guide = new Clickable(name, pos, height, width, action);
        this.state.clickables[name] = guide;
        if (!this.displayDriver.images.has(name)) {
            this.displayDriver.loadImage(name, asset);
        }
    }

    private move() {
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

}
