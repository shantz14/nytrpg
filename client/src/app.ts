import { Game } from "./game.js";

const canvas = document.getElementById("game") as HTMLCanvasElement;
if (canvas.getContext) {
    const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;

    const game = new Game(ctx);
    game.run();
} else {
    console.log("No canvas support...");
}

