import { Game } from "./game.js";
import { login, UserData } from "./login.js"
import { selectCharacter } from "./characters.js";

const canvas = document.getElementById("game") as HTMLCanvasElement;
if (canvas.getContext) {
    const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;

    let userData: UserData;
    userData = await login();
    const character = await selectCharacter(userData);
    const game = new Game(ctx, userData, character);
    game.run();
} else {
    console.log("No canvas support...");
}
