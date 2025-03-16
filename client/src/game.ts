import { DisplayDriver } from "./display-driver.js";
import { GameState } from "./game-objects.js";

export class Game {
    displayDriver: DisplayDriver;

    constructor(ctx: CanvasRenderingContext2D) {
        const canvas = ctx.canvas;

        var state = new GameState();

        this.displayDriver = new DisplayDriver(ctx, state);
    }

    public run() {
        this.displayDriver.draw();
    }
}
