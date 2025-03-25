import { GameState } from "./game-objects.js";
import { Vector2D } from "./vector2D.js";

export class DisplayDriver {
    ctx: CanvasRenderingContext2D;
    canvas: HTMLCanvasElement;
    state: GameState;
    images: Map<string, HTMLImageElement>;

    constructor(ctx: CanvasRenderingContext2D, startState: GameState) {
        this.ctx = ctx;
        this.canvas = ctx.canvas;
        this.state = startState;
        this.images = new Map();

        this.scaleCanvas();

        this.loadImages();

        this.draw();
    }

    public draw() {
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        this.drawBackground();
        this.drawCharacter();
    }

    private drawBackground() {
        // TODO: Make image loading not a ton of if elses actually maybe not
        const bg = this.images.get("bg") as HTMLImageElement;
        
        if (bg) {
            this.ctx.drawImage(bg, 0, 0);
        }
    }

    private drawCharacter() {
        const charVec = this.state.charVec;

        const character = this.images.get("character") as HTMLImageElement;

        if (character) {
            this.ctx.drawImage(character, charVec.x, charVec.y);
        }
    }

    private scaleCanvas() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
    }

    private loadImages() {
        this.loadImage("character", "Skoobyuboo.png");

        this.loadImage("bg", "background.jpg");
    }

    private loadImage(key: string, filename: string) {
        const path: string = "./assets/" + filename;

        const image = new Image();
        image.src = path;
        image.onload = () => {
            this.images.set(key, image);
        }
    }
}
