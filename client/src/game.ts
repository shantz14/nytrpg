import { DisplayDriver } from "./display-driver.js";
import { InputDriver } from "./input-driver.js";
import { GameState } from "./game-objects.js";

export class Game {
    displayDriver: DisplayDriver;
    inputDriver: InputDriver;
    state: GameState;

    constructor(ctx: CanvasRenderingContext2D) {
        const canvas = ctx.canvas;

        this.state = new GameState();

        this.inputDriver = new InputDriver(this.state);
        this.displayDriver = new DisplayDriver(ctx, this.state);
    }

    public update() {

    }

    public run() {
        //this.displayDriver.draw();
        let me = this;
        setInterval(function() {
            me.displayDriver.draw();
        }, 33);
    }
}
