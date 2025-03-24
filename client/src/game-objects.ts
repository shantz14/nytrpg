import { Vector2D } from "./vector2d.js"

export class GameState {
    charVec: Vector2D;

    constructor() {
        this.charVec = new Vector2D(window.innerWidth/2, window.innerHeight/2);
    }
}
