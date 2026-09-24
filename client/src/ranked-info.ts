import { Popup } from "./popup.js";
import { InputDriver } from "./input-driver.js";
import { GameState } from "./game-objects.js";
import { RankTier, Stakes } from "./protocol.gen.js";
import { className } from "./classes.js";
import { signed, tierFor } from "./ranks.js";

// A player's rank as a colored badge: "Gold 2"
export function rankBadge(tier: RankTier | null, elo?: number): HTMLSpanElement {
    const el = document.createElement("span");
    el.className = "rank-badge" + (tier ? " rank-" + tier.family : "");
    el.textContent = (tier?.name ?? "Unranked") + (elo === undefined ? "" : ` · ${elo}`);
    return el;
}

// One side of a ranked matchup
export type Duelist = { char: string, cls: string, elo: number };

export type RankedInfoOptions = {
    them: Duelist,
    stakes: Stakes,
    // e.g. "Send ranked challenge"
    confirm: string,
    onConfirm: () => void,
};

// Explains ranked duels and what's at stake before one starts. Closes any
// other popup first. Returns the popup so the caller can close it, e.g. if
// the challenge expires meanwhile.
export function showRankedInfo(state: GameState, input: InputDriver, opts: RankedInfoOptions): Popup {
    Popup.current?.close();
    const popup = Popup.open("tpl-ranked-info", input)!;
    const q = <T extends HTMLElement>(sel: string) => popup.q<T>(sel);

    const side = (el: HTMLElement, d: Duelist) => {
        const name = document.createElement("span");
        name.className = "ri-name";
        // textContent, never innerHTML: names come from users
        name.textContent = d.char;
        const cls = document.createElement("span");
        cls.className = "ri-class class-" + d.cls;
        cls.textContent = className(state.classes, d.cls);
        el.replaceChildren(name, cls, rankBadge(tierFor(d.elo, state.ladder), d.elo));
    };
    side(q("#riYou"), { char: state.selfChar, cls: state.selfClass?.id ?? "", elo: state.selfElo });
    side(q("#riThem"), opts.them);

    const s = opts.stakes;
    q("#riWin").textContent = `${signed(s.winMin)} to ${signed(s.winMax)}`;
    q("#riLose").textContent = `${signed(s.loseMin)} to ${signed(s.loseMax)}`;

    // The whole ladder, best at the top, with your rank marked
    const mine = tierFor(state.selfElo, state.ladder);
    const ladder = q("#riLadder");
    ladder.replaceChildren();
    [...state.ladder].reverse().forEach((tier, i, all) => {
        const row = document.createElement("li");
        row.className = "ri-tier rank-" + tier.family + (tier.id === mine?.id ? " mine" : "");
        const name = document.createElement("span");
        name.className = "ri-tier-name";
        name.textContent = tier.name;
        const range = document.createElement("span");
        range.className = "ri-tier-range";
        const above = all[i - 1];
        range.textContent = above ? `${tier.minElo}–${above.minElo - 1}` : `${tier.minElo}+`;
        row.append(name, range);
        ladder.appendChild(row);
    });

    const confirm = q<HTMLButtonElement>("#riConfirm");
    confirm.textContent = opts.confirm;
    popup.on(confirm, "click", () => {
        popup.close();
        opts.onConfirm();
    });
    confirm.focus();
    return popup;
}
