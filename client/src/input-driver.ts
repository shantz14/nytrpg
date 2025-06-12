import { Vector2D } from "./vector2D.js";
import { GameState } from "./game-objects.js";

export class InputDriver {
    canvas: HTMLCanvasElement;
    keysPressed: Set<string>;
    mousePos: Vector2D;
    state: GameState;
    gameFocused: boolean;

    constructor(canvas: HTMLCanvasElement, state: GameState) {
        this.canvas = canvas;
        this.keysPressed = new Set();
        this.mousePos = new Vector2D(0, 0);
        this.state = state;
        this.gameFocused = true;

        document.addEventListener('keydown', (event) => {
            const key = event.key.toLowerCase();
            this.keysPressed.add(key);
        });

        document.addEventListener('keyup', (event) => {
            const key = event.key.toLowerCase();
            this.keysPressed.delete(key);
        });

        document.addEventListener('mousemove', (event) => {
            this.updateMousePos(event);
        });

        document.addEventListener('mouseup', (event) => {
            this.click(event);
        });
    }

    private click(e: MouseEvent) {
        for (const key in this.state.clickables) {
            const obj = this.state.clickables[key];
            if (obj.rect.inRect(this.mousePos) && this.gameFocused) {
                obj.action();
            }
        }
    }

    private updateMousePos(e: MouseEvent) {
        let rect = this.canvas.getBoundingClientRect();

        let x = e.clientX - rect.left;
        let y = e.clientY - rect.top;

        this.mousePos.set(x, y);
    }
}
