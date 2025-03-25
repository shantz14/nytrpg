import { Vector2D } from "./vector2D.js";
import { GameState } from "./game-objects.js";

export class InputDriver {
    state: GameState;
    keysPressed: Set<string>;

    constructor(state: GameState) {
        this.state = state;
        this.keysPressed = new Set();

        document.addEventListener('keydown', (event) => {
            const key = event.key.toLowerCase();
            this.keysPressed.add(key);
        });

        document.addEventListener('keyup', (event) => {
            const key = event.key.toLowerCase();
            this.keysPressed.delete(key);
        });
    }
}
