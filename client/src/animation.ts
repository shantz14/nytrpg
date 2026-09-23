// Sprite animation. No DOM here, so it can be unit tested in Node.

// Frames laid out left to right in one image
export type SpriteSheet = {
    image: string;
    frameWidth: number;
    frameHeight: number;
    frames: number;
    fps: number;
    // Which way the art faces. The other way is drawn mirrored.
    facesRight: boolean;
}

export type SpriteAnimations = {
    // Standing still
    idle: string;
    walk: SpriteSheet;
}

// Animations by the sprite name the server sends for an entity. Sprites not
// listed here are drawn as a plain image.
export const ANIMATIONS: Record<string, SpriteAnimations> = {
    "Skoobyuboo.png": {
        idle: "Skoobyuboo.png",
        walk: { image: "player-walk.png", frameWidth: 64, frameHeight: 64, frames: 5, fps: 10, facesRight: true },
    },
};

// Keep walking this long after the last movement. Other players' positions
// arrive in bursts, without this they'd flicker to idle between updates.
export const STOP_GRACE_S = 0.15;

// Tracks whether an entity is walking and which way it faces, from how it moves
export class Animator {
    // +1 right, -1 left
    facing: number;
    // Seconds spent walking since it last stood still
    walkTime: number;
    // Seconds since it last moved
    stillFor: number;

    constructor() {
        this.facing = 1;
        this.walkTime = 0;
        this.stillFor = Infinity;
    }

    // dx, dy: how far it moved during the last dt seconds
    public update(dx: number, dy: number, dt: number) {
        const wasWalking = this.walking;
        if (dx !== 0) {
            this.facing = dx > 0 ? 1 : -1;
        }
        if (dx !== 0 || dy !== 0) {
            this.stillFor = 0;
        } else {
            this.stillFor += dt;
        }

        // A new walk starts on its first frame
        if (this.walking && wasWalking) {
            this.walkTime += dt;
        } else {
            this.walkTime = 0;
        }
    }

    get walking(): boolean {
        return this.stillFor <= STOP_GRACE_S;
    }

    // Which frame of the sheet to draw now
    public frame(sheet: SpriteSheet): number {
        // The epsilon stops float drift (0.1 + 0.1 + ... = 0.7999...) landing a frame short
        return Math.floor(this.walkTime * sheet.fps + 1e-6) % sheet.frames;
    }

    // Whether the sheet has to be mirrored to face the right way
    public mirrored(sheet: SpriteSheet): boolean {
        return (this.facing > 0) !== sheet.facesRight;
    }
}

// What to draw for an entity right now
export type Pose =
    | { image: string }
    | { image: string, frame: number, sheet: SpriteSheet, mirrored: boolean };

export function pose(sprite: string, anim: Animator): Pose {
    const anims = ANIMATIONS[sprite];
    if (!anims) {
        return { image: sprite };
    }
    if (!anim.walking) {
        return { image: anims.idle };
    }
    const sheet = anims.walk;
    return { image: sheet.image, frame: anim.frame(sheet), sheet, mirrored: anim.mirrored(sheet) };
}
