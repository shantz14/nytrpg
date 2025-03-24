import { GameState } from "./game-objects.js";
import { Vector2D } from "./vector2D.js";

export class DisplayDriver {
    ctx: CanvasRenderingContext2D;
    canvas: HTMLCanvasElement;
    state: GameState;

    constructor(ctx: CanvasRenderingContext2D, startState: GameState) {
        this.ctx = ctx;
        this.canvas = ctx.canvas;
        
        this.state = startState;

        this.scaleCanvas();

        this.draw();
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
        image.onload = function() {
            ctx.drawImage(image, 0, 0);
        }

    }

    private drawCharacter() {
        const image = new Image();
        const ctx = this.ctx;

        const charVec = this.state.charVec;

        image.src = "./assets/Skoobyuboo.png";
        image.onload = function() {
            ctx.drawImage(image, charVec.x, charVec.y);
        }
    }

    private scaleCanvas() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
    }
}
