import { InputDriver } from "./input-driver.js";

export type HudActions = {
    abilities: () => void;
    profile: () => void;
    leaderboard: () => void;
    characters: () => void;
    logout: () => void;
};

// Shows the buttons fixed to the screen and wires them up. The ones that open
// popups only work while the game has focus, so one can't open a popup over
// another. The ability panel is a side panel, it always toggles.
export function mountHud(actions: HudActions, input: InputDriver) {
    const hud = document.getElementById("hud")!;
    const buttons: [string, () => void, boolean][] = [
        ["hudAbilities", actions.abilities, false],
        ["hudProfile", actions.profile, true],
        ["hudLeaderboard", actions.leaderboard, true],
        ["hudCharacters", actions.characters, true],
        ["hudLogout", actions.logout, true],
    ];
    for (const [id, action, popup] of buttons) {
        const btn = document.getElementById(id)!;
        btn.addEventListener("click", () => {
            // Keyboard focus would let Space or Enter click it again
            btn.blur();
            if (!popup || input.isGameFocused()) {
                action();
            }
        });
    }
    hud.hidden = false;
}
