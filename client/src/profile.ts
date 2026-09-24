import { Popup } from "./popup.js";
import { InputDriver } from "./input-driver.js";
import { GameState } from "./game-objects.js";
import { DuelDraw, DuelWin, Profile } from "./protocol.gen.js";
import { className } from "./classes.js";
import { nextTier, signed, tierFor, tierProgress } from "./ranks.js";
import { rankBadge } from "./ranked-info.js";

// A player's ranked profile. Opens right away saying it's loading, and fills
// in when the server's answer arrives (show). Returns null if another popup
// is open.
export class ProfileView {
    private popup: Popup;

    private constructor(popup: Popup, private state: GameState) {
        this.popup = popup;
    }

    static open(state: GameState, input: InputDriver): ProfileView | null {
        const popup = Popup.open("tpl-profile", input);
        return popup ? new ProfileView(popup, state) : null;
    }

    get closed(): boolean {
        return !this.popup.root.isConnected;
    }

    public show(p: Profile) {
        if (this.closed) {
            return;
        }
        const q = <T extends HTMLElement>(sel: string) => this.popup.q<T>(sel);
        const ladder = this.state.ladder;
        // textContent, never innerHTML: names come from users
        q("#pfChar").textContent = p.char || p.name;
        const cls = q("#pfClass");
        cls.textContent = p.class ? className(this.state.classes, p.class) : "";
        cls.className = "pf-class" + (p.class ? " class-" + p.class : "");
        q("#pfUser").textContent = p.char ? p.name : "";

        const tier = tierFor(p.elo, ladder);
        const rank = q("#pfRank");
        rank.textContent = tier?.name ?? "";
        rank.className = "pf-rank" + (tier ? " rank-" + tier.family : "");
        q("#pfElo").textContent = `${p.elo} elo`;
        const next = nextTier(p.elo, ladder);
        q("#pfNext").textContent = next ? `${next.minElo - p.elo} to ${next.name}` : "Top of the ladder";
        const bar = q("#pfBar");
        bar.style.setProperty("--progress", String(tierProgress(p.elo, ladder)));
        bar.className = "pf-bar" + (tier ? " rank-" + tier.family : "");

        const decided = p.wins + p.losses;
        q("#pfGames").textContent = String(p.games);
        q("#pfRecord").textContent = `${p.wins}–${p.losses}–${p.draws}`;
        q("#pfRate").textContent = decided ? Math.round(p.wins / decided * 100) + "%" : "–";
        const peak = rankBadge(tierFor(p.peak, ladder));
        peak.title = `${p.peak} elo`;
        q("#pfPeak").replaceChildren(peak);

        const list = q("#pfRecent");
        list.replaceChildren();
        for (const m of p.recent ?? []) {
            const row = document.createElement("li");
            const result = m.outcome == DuelWin ? "win" : m.outcome == DuelDraw ? "draw" : "loss";
            row.className = "pf-match " + result;
            const res = document.createElement("span");
            res.className = "pf-result";
            res.textContent = result === "win" ? "W" : result === "draw" ? "D" : "L";
            const vs = document.createElement("span");
            vs.className = "pf-vs";
            vs.textContent = "vs ";
            const opp = document.createElement("span");
            opp.className = "pf-opp class-" + m.opponentClass;
            opp.textContent = m.opponent;
            vs.appendChild(opp);
            const change = document.createElement("span");
            change.className = "pf-change";
            change.textContent = signed(m.change);
            row.append(res, vs, change);
            list.appendChild(row);
        }
        q("#pfNoGames").hidden = p.games > 0;
        this.popup.root.classList.remove("loading");
    }
}
