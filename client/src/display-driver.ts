import { GameState } from "./game-objects.js";

export class DisplayDriver {
    ctx: CanvasRenderingContext2D;
    canvas: HTMLCanvasElement;

    constructor(ctx: CanvasRenderingContext2D, state: GameState) {
        this.ctx = ctx;
        this.canvas = ctx.canvas;
    }

    public draw() {
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        this.drawBackground();
        this.drawCharacter();
    }

    private drawBackground() {
        const image = new Image();
        const ctx = this.ctx;

        image.src = "./assets/background.jpg";
        console.log("Loading the image");
        image.onload = function() {
            ctx.drawImage(image, 0, 0);
        }

    }

    private drawCharacter() {

    }
}
