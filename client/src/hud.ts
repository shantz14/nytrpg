import { InputDriver } from "./input-driver.js";

export type HudActions = {
    leaderboard: () => void;
    logout: () => void;
};

// Shows the buttons fixed to the screen and wires them up. They only work
// while the game has focus, so one can't open a popup over another.
export function mountHud(actions: HudActions, input: InputDriver) {
    const hud = document.getElementById("hud")!;
    const buttons: [string, () => void][] = [
        ["hudLeaderboard", actions.leaderboard],
        ["hudLogout", actions.logout],
    ];
    for (const [id, action] of buttons) {
        const btn = document.getElementById(id)!;
        btn.addEventListener("click", () => {
            // Keyboard focus would let Space or Enter click it again
            btn.blur();
            if (input.isGameFocused()) {
                action();
            }
        });
    }
    hud.hidden = false;
}
