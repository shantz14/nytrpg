export class Vector2D {
    x: number;
    y: number;

    constructor(x: number, y: number) {
        this.x = x;
        this.y = y;
    }

    public add(otherVec: Vector2D) {
        this.x += otherVec.x;
        this.y += otherVec.y;
    }

    public subtract(otherVec: Vector2D) {
        this.x -= otherVec.x;
        this.y -= otherVec.y;
    }

    public equals(otherVec: Vector2D): boolean {
        return (this.x == otherVec.x && this.y == otherVec.y);
    }
}
