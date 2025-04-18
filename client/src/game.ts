import { DisplayDriver } from "./display-driver.js";
import { InputDriver } from "./input-driver.js";
import { GameState } from "./game-objects.js";
import { Vector2D } from "./vector2D.js";

const SERVER_URL = "http://localhost:8080/ws";

export class Game {
    sock: WebSocket;
    displayDriver: DisplayDriver;
    inputDriver: InputDriver;
    state: GameState;

    constructor(ctx: CanvasRenderingContext2D) {
        const canvas = ctx.canvas;

        // WebSocket or WebSocketStream?
        this.sock = new WebSocket(SERVER_URL);

        this.state = new GameState();

        this.inputDriver = new InputDriver(this.state);
        this.displayDriver = new DisplayDriver(ctx, this.state);
    }

    public run() {
        setInterval(() => {
            this.update();
        }, 33);
    }

    private update() {
        this.move();
        this.displayDriver.draw();
    }

    private move() {
        let movement = new Vector2D(0, 0); 

        if (this.inputDriver.keysPressed.has("w")) {
            movement.y = -15;
        }
        else if (this.inputDriver.keysPressed.has("s")) {
            movement.y = 15;
        }

        if (this.inputDriver.keysPressed.has("a")) {
            movement.x = -15;
        }
        else if (this.inputDriver.keysPressed.has("d")) {
            movement.x = 15;
        }

        this.state.charVec.add(movement);
    }

}
