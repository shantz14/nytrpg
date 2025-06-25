import { GameState } from "./game-objects.js";
import { UserData } from "./login.js";
import { Vector2D } from "./vector2D.js";

export class DisplayDriver {
    ctx: CanvasRenderingContext2D;
    canvas: HTMLCanvasElement;
    state: GameState;
    images: Map<string, HTMLImageElement>;
    userData: UserData;

    constructor(ctx: CanvasRenderingContext2D, startState: GameState, userData: UserData) {
        this.ctx = ctx;
        this.canvas = ctx.canvas;
        this.state = startState;
        this.images = new Map();
        this.userData = userData;

        this.scaleCanvas();

        this.loadImages();

        this.draw();
    }

    public draw() {
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        this.drawBackground();
        this.drawCharacter();
        this.drawOtherChars();
    }

    private drawBackground() {
        const bg = this.images.get("bg") as HTMLImageElement;
        
        if (bg) {
            this.ctx.drawImage(bg, 0, 0);
        }
    }

    private drawCharacter() {
        const charVec = this.state.charVec;
        const sprite = this.images.get("character") as HTMLImageElement;
        if (sprite) {
            this.ctx.drawImage(sprite, charVec.x, charVec.y);
            this.ctx.font = "26px serif"
            this.ctx.fillText(this.userData.username, charVec.x, charVec.y-10);
        }
    }

    private drawOtherChars() {
        for (const id in this.state.otherChars) {
            const charVec = this.state.otherChars[id].pos;
            const sprite = this.images.get(String(id)) as HTMLImageElement;

            if (sprite) {
                this.ctx.drawImage(sprite, charVec.x, charVec.y);
            }
        }

        for (const name in this.state.clickables) {
            const pos = this.state.clickables[name].rect.tl;
            const sprite = this.images.get(name) as HTMLImageElement;

            if (sprite) {
                this.ctx.drawImage(sprite, pos.x, pos.y);
            }
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

    public loadImage(key: string, filename: string) {
        const path: string = "./assets/" + filename;

        const image = new Image();
        image.src = path;
        image.onload = () => {
            this.images.set(key, image);
        }
    }
}
