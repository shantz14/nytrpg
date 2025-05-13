import { Vector2D } from "./vector2D.js"

export class GameState {
    charVec: Vector2D;
    otherChars: {[key: number]: PlayerState};
    clickables: {[key: string]: Clickable};

    constructor() {
        this.charVec = new Vector2D(window.innerWidth/2, window.innerHeight/2);
        this.otherChars = {};
        this.clickables = {};
    }
}

export class UpdateState {
    players: {[key: string]: PlayerState};
    unregister: number

    constructor() {
        this.players = {};
        this.unregister = -999;
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

export class Clickable {
    name: string;
    rect: Rect;
    action: Function;

    constructor(name: string, pos: Vector2D, height: number, width: number, action: Function) {
        this.name = name;
        this.rect = new Rect(pos, height, width);
        this.action = action;
    }
}

// x1, y1 - - - |
// |            |
// | - - - x2, y2
export class Rect {
    tl: Vector2D;
    br: Vector2D;
    pos: Vector2D;
    height: number;
    width: number;

    constructor(pos: Vector2D, height: number, width: number) {
        this.pos = pos;
        this.height = height;
        this.width = width;
        this.tl = new Vector2D(pos.x, pos.y);
        this.br = new Vector2D(pos.x + width, pos.y + height);
    }

    // True if position passed is inside the rect
    public inRect(mouse: Vector2D) {
        if (mouse.x >= this.tl.x && mouse.x <= this.br.x && mouse.y >= this.tl.y && mouse.y <= this.br.y) {
            return true;
        }
        return false;
    }
}
