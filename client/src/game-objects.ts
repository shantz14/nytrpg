import { Vector2D } from "./vector2D.js"

export class GameState {
    charVec: Vector2D;

    otherChars: {[key: number]: PlayerState};

    constructor() {
        this.charVec = new Vector2D(window.innerWidth/2, window.innerHeight/2);
        this.otherChars = {};
    }
}

export class UpdateState {
    players: {[key: string]: PlayerState};

    constructor() {
        this.players = {};
    }
}

export class PlayerState {
    id: number;
    pos: Vector2D;
    me: boolean;

    constructor() {
        this.id = -1;
        this.pos = new Vector2D(0, 0); 
        this.me = false;
    }
}
