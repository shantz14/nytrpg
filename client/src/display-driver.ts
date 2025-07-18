import { GameState } from "./game-objects.js";
import { UserData } from "./login.js";
import { Chat } from "./messages.js";
import { Vector2D } from "./vector2D.js";

export class DisplayDriver {
    ctx: CanvasRenderingContext2D;
    canvas: HTMLCanvasElement;
    state: GameState;
    images: Map<string, HTMLImageElement>;
    chats: Map<number, ChatData>;
    userData: UserData;
    middle: Vector2D;

    constructor(ctx: CanvasRenderingContext2D, startState: GameState, userData: UserData, middle: Vector2D) {
        this.ctx = ctx;
        this.canvas = ctx.canvas;
        this.state = startState;
        this.images = new Map();
        this.chats = new Map();
        this.userData = userData;
        this.middle = middle;

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
        const pP = this.state.charVec;
        
        if (bg) {
            this.ctx.drawImage(bg, 0-pP.x, 0-pP.y);
        }
    }

    private drawCharacter() {
        const sprite = this.images.get("character") as HTMLImageElement;
        if (sprite) {
            this.ctx.drawImage(sprite, this.middle.x, this.middle.y);
            this.ctx.font = "26px serif";
            this.ctx.fillText(this.userData.username, this.middle.x, this.middle.y-10);
            let chatdata = this.chats.get(this.userData.id);
            if (chatdata && this.chatIsExpired(chatdata)) {
                this.chats.delete(this.userData.id);
                chatdata = undefined;
            }
            if (chatdata) {
                this.ctx.fillText(chatdata.chat.msg, this.middle.x, this.middle.y-35);
            }
        }
    }

    private drawOtherChars() {
        for (const id in this.state.otherChars) {
            const charVec = this.state.otherChars[id].pos;
            const username = this.state.otherChars[id].username;
            const sprite = this.images.get(String(id)) as HTMLImageElement;

            const adjusted = new Vector2D(charVec.x, charVec.y);
            adjusted.subtract(this.state.charVec);
            if (sprite) {
                this.ctx.drawImage(sprite, adjusted.x, adjusted.y);
                this.ctx.font = "26px serif";
                this.ctx.fillText(username, adjusted.x, adjusted.y-10);
                let chatdata = this.chats.get(Number(id));
                if (chatdata && this.chatIsExpired(chatdata)) {
                    this.chats.delete(Number(id));
                    chatdata = undefined;
                }
                if (chatdata) {
                    this.ctx.fillText(chatdata.chat.msg, adjusted.x, adjusted.y-35);
                }
            }
        }

        for (const name in this.state.clickables) {
            const sprite = this.images.get(name) as HTMLImageElement;
            this.state.clickables[name].rect.adjust(this.state.charVec);
            const pos = this.state.clickables[name].rect.tl;

            if (sprite) {
                this.ctx.drawImage(sprite, pos.x, pos.y);
            }
        }
    }

    private chatIsExpired(chatData: ChatData): boolean {
        return Date.now() > chatData.exp;
    }

    public updateChat(chat: Chat) {
        let pos: Vector2D;
        if (chat.id == this.userData.id) {
            pos = this.state.charVec;
        } else {
            pos = this.state.otherChars[chat.id].pos;
        }
        let exp = Date.now();
        exp += 5 * 1000; // Add 5 seconds
        this.chats.set(chat.id, {
            chat: chat,
            exp: exp
        });
    }

    private scaleCanvas() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;

        window.addEventListener("resize", () => {
            this.canvas.width = window.innerWidth;
            this.canvas.height = window.innerHeight;
        });
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

type ChatData = {
    chat: Chat,
    exp: number
}

