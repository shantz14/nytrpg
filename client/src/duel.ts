import { Game } from "./game.js";
import { Popup } from "./popup.js";
import { OpponentGrid, WordleBoard } from "./wordle-board.js";
import { Stopwatch, renderSolution } from "./wordle.js";
import { LatestThrottle } from "./throttle.js";
import { className } from "./classes.js";
import { signed, tierFor } from "./ranks.js";
import { rankBadge } from "./ranked-info.js";
import { AbilityBar, PICKPOCKET, PURIFY, castable, iconUrl, validPattern } from "./abilities.js";
import { tileClass } from "./wordle-board.js";
import {
    AbilityInfo, CastFizzled, CastLanded, CastTriggered, ClassInfo, ClientDuelCast, ClientDuelForfeit, ClientDuelGuess,
    ClientDuelReshape, ClientDuelTyping, ClientIllusionGuess, DuelBoard, DuelCast, DuelCastReq, DuelDisconnect, DuelDivine,
    DuelDraw, DuelEnd, DuelEyes, DuelForfeit, DuelGuessRemoved, DuelOpponentGuess, DuelOutOfGuesses, DuelPickpocket,
    DuelReshapeOptions, DuelReveal, DuelScry, DuelSideState, DuelSolved, DuelStart, DuelState, DuelTimeUp, DuelTyping,
    DuelWin, FateCleanSlate, Green, Grey, IllusionEnd, IllusionStart, TargetColors, TargetLetter, TargetOpponentTile,
    TargetOwnRow, TargetWord, WordleColor, WordleLose, WordleRes, WordleWin, Yellow,
} from "./protocol.gen.js";
import { proclaim } from "./prayer-window.js";

// Typing updates go out at most this often
const TYPING_INTERVAL_MS = 50;
// Stun and missile countdowns redraw this often
const COUNTDOWN_MS = 100;
// Matches the server's MissileFlight
const MISSILE_FLIGHT_MS = 15000;
// Energy pips shown, the most any ability costs
const ENERGY_PIPS = 12;
// Lines kept in the event feed
const FEED_LINES = 4;
// How long an escaped or failed Illusion stays up before closing
const ILLUSION_END_MS = 1600;

const DEFAULT_HINT = "Enter submits · Backspace clears the row";

// What a passive did, for the event feed: "<Passive>: <opponent> <did>" and
// "<opponent>'s <Passive> <did to you>"
const PASSIVE_EFFECTS: Record<string, [string, string]> = {
    aggressive: ["is stunned", "stunned you"],
    sneaky: ["has two keys swapped", "swapped two of your keys"],
    divine_will: ["is silenced", "silenced you"],
};

const ORDINALS = ["1st", "2nd", "3rd", "4th", "5th", "6th"];

type Side = "you" | "them";

// An ability or passive by ID, whichever class has it
function findAbility(classes: ClassInfo[], id: string): { name: string; icon: string } | null {
    for (const c of classes) {
        const a = c.abilities.find((a) => a.id === id);
        if (a) {
            return a;
        }
        if (c.passive.id === id) {
            return c.passive;
        }
    }
    return null;
}

// A duel in progress: your board and the opponent's colors side by side,
// with energy, abilities and their effects
export class Duel {
    private game: Game;
    private start: DuelStart;
    private popup: Popup | null;
    private board: WordleBoard | null;
    private opponent: OpponentGrid | null;
    private stopwatch: Stopwatch | null;
    private typing: LatestThrottle<number>;
    private ended: boolean;
    // Out of guesses, waiting (or for Determination)
    private out: boolean;
    private bar: AbilityBar | null;
    // The latest energy and effects, and when it came in (performance.now())
    private state: DuelState | null;
    private stateAt: number;
    private countdown: number | undefined;
    // Letters scried, and whether each is in the word
    private scried: Map<string, boolean>;
    private layer: HTMLElement | null;
    private illusion: { board: WordleBoard; layer: HTMLElement } | null;

    constructor(game: Game, start: DuelStart) {
        this.game = game;
        this.start = start;
        this.popup = null;
        this.board = null;
        this.opponent = null;
        this.stopwatch = null;
        this.ended = false;
        this.out = false;
        this.bar = null;
        this.state = null;
        this.stateAt = 0;
        this.scried = new Map();
        this.layer = null;
        this.illusion = null;
        this.typing = new LatestThrottle<number>((count) => {
            const t: DuelTyping = { count };
            this.game.send(ClientDuelTyping, t);
        }, TYPING_INTERVAL_MS);
    }

    // The opponent: character name, or username if they have none
    private get opponentName(): string {
        return this.start.char || this.start.name;
    }

    // Opens the duel window, closing whatever popup was open
    public open() {
        Popup.current?.close();
        const popup = Popup.open("tpl-duel", this.game.inputDriver, false)!;
        this.popup = popup;
        popup.onCleanup(() => {
            this.stopwatch?.stop();
            this.typing.cancel();
            clearInterval(this.countdown);
            this.opponent?.cancelPick();
        });
        popup.onClose = () => {
            if (this.game.duel === this) {
                this.game.duel = null;
            }
        };

        const s = this.start;
        popup.q("#duelVsName").textContent = this.opponentName;
        popup.q("#duelRankedTag").hidden = !s.ranked;
        const cls = popup.q("#duelVsClass");
        cls.textContent = s.class ? className(this.game.state.classes, s.class) : "";
        cls.className = "duel-vs-class" + (s.class ? " class-" + s.class : "");
        popup.q("#duelOppLabel").textContent = this.opponentName;

        const board = new WordleBoard(popup, popup.q("#duelBoard"), popup.q("#duelSubmit"), {
            wordLength: s.wordLength,
            rows: s.maxGuesses,
            idPrefix: "duel-",
        });
        board.onSubmit = (guess) => this.game.send(ClientDuelGuess, { guess });
        board.onTypingChange = (count) => this.typing.push(count);
        this.board = board;
        board.focus();
        this.opponent = new OpponentGrid(popup.q("#duelOpponent"), s.wordLength, s.maxGuesses);
        this.updateOpponentStatus();

        this.bar = new AbilityBar(popup.q("#duelAbilityBar"), this.game.state.selfClass, (slot, a) => this.use(slot, a));
        this.bar.update(null);
        popup.on(popup.q("#duelAbilityInfo"), "click", () => {
            this.game.abilityPanel?.toggle();
            this.board?.focus();
        });
        popup.on(document, "keydown", (e) => this.onKey(e as KeyboardEvent));

        this.stopwatch = new Stopwatch(popup.q("#duelTimer"));
        this.stopwatch.start();
        this.countdown = setInterval(() => this.tickCountdowns(), COUNTDOWN_MS);

        popup.on(popup.q("#duelForfeit"), "click", () => this.confirmForfeit());
    }

    // 1-5 cast, Escape cancels picking a target
    private onKey(e: KeyboardEvent) {
        if (e.key === "Escape") {
            if (this.opponent?.isPicking) {
                this.opponent.cancelPick();
            } else if (this.board?.isPickingRow) {
                this.board.cancelPickRow();
            } else if (this.layer) {
                this.closeLayer();
            }
            return;
        }
        const slot = Number(e.key) - 1;
        if (e.key.length === 1 && slot >= 0 && slot < 5 && !this.layer && !e.repeat) {
            const a = this.game.state.selfClass?.abilities[slot];
            if (a?.id) {
                e.preventDefault();
                this.use(slot, a);
            }
        }
    }

    // Casts the ability in slot, picking its target first if it needs one
    private async use(slot: number, a: AbilityInfo) {
        // Only Purify works from inside an Illusion
        if (this.ended || !castable(a, this.state) || (this.illusion && a.id !== PURIFY)) {
            return;
        }
        const opponent = this.opponent!;
        const board = this.board!;
        if (opponent.isPicking || board.isPickingRow) {
            // Clicking it again cancels
            opponent.cancelPick();
            board.cancelPickRow();
            return;
        }
        const req: DuelCastReq = { slot, row: 0, col: 0, letter: "", colors: [] };
        if (a.target === TargetOwnRow) {
            this.bar?.setPicking(slot);
            this.setHint(`Click one of your guesses to ${a.name.toLowerCase()} it · Esc cancels`);
            const row = await board.pickRow();
            this.bar?.setPicking(null);
            this.setHint(null);
            if (row === null || this.ended) {
                board.focus();
                return;
            }
            req.row = row;
            this.game.send(ClientDuelCast, req);
            return;
        } else if (a.target === TargetColors) {
            this.pickColors(slot, a);
            return;
        } else if (a.target === TargetOpponentTile) {
            const yellowOnly = a.id === PICKPOCKET;
            this.bar?.setPicking(slot);
            this.setHint(`Click ${yellowOnly ? "a yellow" : "a"} letter in ${this.opponentName}'s guesses to ${a.name.toLowerCase()} it · Esc cancels`);
            const tile = await opponent.pickTile(yellowOnly);
            this.bar?.setPicking(null);
            this.setHint(null);
            if (!tile || this.ended) {
                this.board?.focus();
                return;
            }
            req.row = tile.row;
            req.col = tile.col;
        } else if (a.target === TargetLetter) {
            this.pickLetter(slot, a);
            return;
        }
        // TargetWord: the server replies with the words to pick from
        this.game.send(ClientDuelCast, req);
        if (a.target !== TargetWord) {
            this.board?.focus();
        }
    }

    // Scry: pick a letter from the alphabet
    private pickLetter(slot: number, a: AbilityInfo) {
        const layer = this.openLayer("tpl-duel-scry");
        layer.querySelector(".panel-title")!.textContent = a.name;
        const grid = layer.querySelector("#scryLetters")!;
        for (const letter of "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "scry-letter";
            btn.textContent = letter;
            const known = this.scried.get(letter);
            if (known !== undefined) {
                btn.classList.add(known ? "tile-yellow" : "tile-grey");
            }
            btn.addEventListener("click", () => {
                this.closeLayer();
                this.game.send(ClientDuelCast, { slot, row: 0, col: 0, letter } as DuelCastReq);
            });
            grid.appendChild(btn);
        }
        (grid.firstElementChild as HTMLElement).focus();
    }

    // Feint and Under Their Nose: click tiles to cycle grey, yellow, green
    private pickColors(slot: number, a: AbilityInfo) {
        const layer = this.openLayer("tpl-duel-colors");
        layer.querySelector("#colorsTitle")!.textContent = a.name;
        layer.querySelector("#colorsText")!.textContent = a.description;
        const cycle: WordleColor[] = [Grey, Yellow, Green];
        const colors: WordleColor[] = Array(this.start.wordLength).fill(Grey);
        const cast = layer.querySelector<HTMLButtonElement>("#colorsCast")!;
        const tiles = layer.querySelector("#colorTiles")!;
        colors.forEach((_, i) => {
            const tile = document.createElement("button");
            tile.type = "button";
            tile.className = "color-tile tile-grey";
            tile.setAttribute("aria-label", `Letter ${i + 1}`);
            tile.addEventListener("click", () => {
                colors[i] = cycle[(cycle.indexOf(colors[i]) + 1) % cycle.length];
                tile.className = "color-tile " + tileClass(colors[i]);
                cast.disabled = !validPattern(colors, this.start.wordLength);
            });
            tiles.appendChild(tile);
        });
        cast.addEventListener("click", () => {
            this.closeLayer();
            const req: DuelCastReq = { slot, row: 0, col: 0, letter: "", colors };
            this.game.send(ClientDuelCast, req);
        });
        (tiles.firstElementChild as HTMLElement).focus();
    }

    private openLayer(templateId: string): HTMLElement {
        this.closeLayer();
        const layer = this.popup!.addLayer(templateId);
        for (const btn of layer.querySelectorAll(".layer-cancel")) {
            btn.addEventListener("click", () => this.closeLayer());
        }
        this.layer = layer;
        return layer;
    }

    private closeLayer() {
        if (this.layer) {
            this.popup?.removeLayer(this.layer);
            this.layer = null;
            this.board?.focus();
        }
    }

    private confirmForfeit() {
        const popup = this.popup;
        if (!popup || this.ended || document.getElementById("duelForfeitPopup")) {
            return;
        }
        const layer = popup.addLayer("tpl-duel-forfeit");
        layer.querySelector("#forfeitName")!.textContent = this.opponentName;
        const close = () => {
            popup.removeLayer(layer);
            this.board?.focus();
        };
        layer.querySelector("#cancelForfeit")!.addEventListener("click", close);
        layer.querySelector("#confirmForfeit")!.addEventListener("click", () => {
            close();
            this.game.send(ClientDuelForfeit, {});
        });
    }

    public handleGuess(res: WordleRes) {
        const board = this.board;
        if (!board || this.ended) {
            return;
        }
        if (res.blocked) {
            board.reject();
            this.game.toast(this.state?.you.illusion ? "You're trapped in an illusion" : "Your keyboard is stunned");
            return;
        }
        if (!res.valid) {
            board.reject();
            this.game.toast("Not in word list");
            return;
        }
        board.colorRow(board.currentGuess - 1, res.colors);
        if (res.status == WordleLose) {
            board.lock();
            this.out = true;
            this.setYouStatus("Out of guesses. Waiting for " + this.opponentName + "…");
        } else if (res.status == WordleWin) {
            board.lock();
        }
    }

    public handleOpponentGuess(g: DuelOpponentGuess) {
        this.opponent?.addGuess(g.colors);
        this.updateOpponentStatus();
    }

    public handleTyping(t: DuelTyping) {
        this.opponent?.setTyping(t.count);
    }

    // Energy and effects on both sides changed
    public handleState(s: DuelState) {
        const popup = this.popup;
        if (!popup || this.ended) {
            return;
        }
        const before = this.state;
        this.state = s;
        this.stateAt = performance.now();

        this.renderEnergy(popup.q("#duelYouEnergy"), s.you.energy, before?.you.energy);
        this.renderEnergy(popup.q("#duelOppEnergy"), s.them.energy, before?.them.energy);
        this.renderEffects(popup.q("#duelYouEffects"), s.you, "you");
        this.renderEffects(popup.q("#duelOppEffects"), s.them, "them");

        const board = this.board!;
        board.setRows(s.you.rows);
        this.opponent!.setRows(s.them.rows);
        // Determination after running out of guesses
        if (this.out && s.you.rows > s.you.guesses) {
            this.out = false;
            board.unlock();
            this.setYouStatus(DEFAULT_HINT);
        }
        board.setDisabled(s.you.stunnedMs > 0 || s.you.illusion);
        board.setKeymap(s.you.keymap);
        this.illusion?.board.setKeymap(s.you.keymap);
        popup.q("#duelAbilityBar").classList.toggle("silenced", s.you.silencedMs > 0);
        popup.q("#duelDeadline").hidden = s.deadlineMs <= 0;
        this.bar?.update(s);
        this.updateOpponentStatus();
        this.tickCountdowns();
    }

    private renderEnergy(el: HTMLElement, energy: number, before: number | undefined) {
        el.querySelector(".energy-count")!.textContent = String(energy);
        el.setAttribute("aria-label", `${energy} energy`);
        el.dataset.energy = String(energy);
        const pips = el.querySelector(".energy-pips")!;
        if (pips.childElementCount === 0) {
            for (let i = 0; i < ENERGY_PIPS; i++) {
                pips.appendChild(document.createElement("i"));
            }
        }
        [...pips.children].forEach((pip, i) => pip.classList.toggle("full", i < energy));
        el.classList.toggle("overflow", energy > ENERGY_PIPS);
        if (before !== undefined && energy !== before) {
            const cls = energy > before ? "gain" : "loss";
            el.classList.remove("gain", "loss");
            void el.offsetWidth;
            el.classList.add(cls);
        }
    }

    // Shield, Seeing Eyes, Illusion and missiles on a side, as chips
    private renderEffects(el: HTMLElement, st: DuelSideState, side: Side) {
        el.replaceChildren();
        const chip = (icon: string, text: string, cls: string) => {
            const c = document.createElement("span");
            c.className = "effect-chip " + cls;
            const img = document.createElement("img");
            img.src = iconUrl("abilities/" + icon + ".png");
            img.alt = "";
            c.append(img, text);
            el.appendChild(c);
            return c;
        };
        if (st.shield) {
            chip("shields_up", "Shield", "effect-shield");
        }
        if (st.eyes > 0) {
            chip("seeing_eye", st.eyes > 1 ? `Seeing Eye ×${st.eyes}` : "Seeing Eye", "effect-eye");
        }
        if (st.illusion) {
            chip("illusion", "In an illusion", "effect-illusion");
        }
        if (st.silencedMs > 0) {
            chip("divine_will", "Silenced", "effect-silenced").dataset.ms = String(st.silencedMs);
        }
        if (st.scrambledGuesses > 0) {
            const n = st.scrambledGuesses;
            chip("confuse", `Scrambled keys · ${n} guess${n === 1 ? "" : "es"}`, "effect-scrambled");
        }
        if (st.cheatReady) {
            chip("cheat", "Next guess: any letters", "effect-ready");
        }
        if (st.feintReady) {
            chip("feint", "Next guess feints", "effect-ready");
        }
        st.missilesMs.forEach((ms) => {
            const c = chip("magic_missile", side === "you" ? "Guess before it lands!" : "Missile", "effect-missile");
            c.dataset.ms = String(ms);
            const bar = document.createElement("span");
            bar.className = "missile-bar";
            bar.appendChild(document.createElement("span"));
            c.appendChild(bar);
        });
        const stun = this.popup!.q(side === "you" ? "#duelYouStun" : "#duelOppStun");
        stun.hidden = st.stunnedMs <= 0;
        stun.dataset.ms = String(st.stunnedMs);
    }

    // Counts stuns and missiles down between states
    private tickCountdowns() {
        const popup = this.popup;
        if (!popup || !this.state) {
            return;
        }
        const elapsed = performance.now() - this.stateAt;
        for (const id of ["#duelYouStun", "#duelOppStun"]) {
            const stun = popup.q(id);
            const left = Number(stun.dataset.ms ?? 0) - elapsed;
            stun.querySelector(".stun-time")!.textContent = `${Math.max(0, Math.ceil(left / 1000))}s`;
        }
        for (const chip of popup.root.querySelectorAll<HTMLElement>(".effect-missile")) {
            const left = Math.max(0, Number(chip.dataset.ms) - elapsed);
            const fill = chip.querySelector<HTMLElement>(".missile-bar span")!;
            fill.style.width = `${100 * (1 - left / MISSILE_FLIGHT_MS)}%`;
        }
        for (const chip of popup.root.querySelectorAll<HTMLElement>(".effect-silenced")) {
            const left = Math.max(0, Number(chip.dataset.ms) - elapsed);
            chip.lastChild!.textContent = `Silenced ${Math.ceil(left / 1000)}s`;
        }
        const deadline = popup.q("#duelDeadline");
        if (!deadline.hidden) {
            const left = Math.max(0, Math.ceil((this.state.deadlineMs - elapsed) / 1000));
            deadline.textContent = `⌛ ${Math.floor(left / 60)}:${String(left % 60).padStart(2, "0")}`;
        }
    }

    // An ability was cast, landed, fizzled or went off
    public handleCast(ev: DuelCast) {
        if (!this.popup || this.ended) {
            return;
        }
        const target: Side = ev.byYou ? "them" : "you";
        const a = findAbility(this.game.state.classes, ev.ability);
        const name = a?.name ?? ev.ability;
        const opp = this.opponentName;

        if (ev.blocked) {
            this.fx(target, "shields_up", "fx-blocked");
            this.feed(ev.byYou ? `${opp}'s shield blocked your ${name}` : `Your shield blocked ${opp}'s ${name}`, "blocked");
            return;
        }

        let text: string;
        switch (ev.kind) {
            case CastLanded:
                if (ev.ability === "magic_missile") {
                    this.missileFx(target);
                    text = ev.byYou ? `Your Magic Missile hit ${opp}` : `A Magic Missile hit you`;
                } else {
                    text = ev.byYou ? `${opp} failed your Illusion` : `You failed the Illusion`;
                }
                break;
            case CastFizzled:
                if (ev.ability === "magic_missile") {
                    text = ev.byYou ? `${opp} guessed in time, your Magic Missile fizzled` : `You guessed in time, the Magic Missile fizzled`;
                } else {
                    text = ev.byYou ? `${opp} saw through your Illusion` : `You escaped the Illusion`;
                }
                break;
            case CastTriggered:
                if (ev.ability === "wise") {
                    text = "Wise: your Seeing Eyes moved";
                    this.fx("you", ev.ability, "fx-cast");
                } else {
                    const [did, didToYou] = PASSIVE_EFFECTS[ev.ability] ?? ["is struck", "struck you"];
                    text = ev.byYou ? `${name}: ${opp} ${did}` : `${opp}'s ${name} ${didToYou}`;
                    this.fx(target, ev.ability, "fx-slam");
                    this.hit(target);
                }
                break;
            default:
                text = ev.byYou ? `You cast ${name}` : `${opp} cast ${name}`;
                this.castFx(ev, target);
        }
        this.feed(text, ev.byYou ? "mine" : "theirs");
    }

    // What a cast looks like
    private castFx(ev: DuelCast, target: Side) {
        switch (ev.ability) {
            case "slash": {
                const tile = ev.byYou ? this.opponent?.tile(ev.row, ev.col) : this.board?.getLetter(ev.row, ev.col);
                tile?.classList.add("slashed");
                setTimeout(() => tile?.classList.remove("slashed"), 700);
                if (ev.byYou) {
                    this.opponent?.destroyTile(ev.row, ev.col);
                } else {
                    this.board?.destroyTile(ev.row, ev.col);
                }
                break;
            }
            case "pommel_strike":
                this.fx(target, ev.ability, "fx-slam");
                this.hit(target);
                break;
            case "cripple":
                this.fx(target, ev.ability, "fx-slam");
                break;
            case "shields_up":
            case "determination":
            case "scry":
                // On the caster
                this.fx(ev.byYou ? "you" : "them", ev.ability, "fx-cast");
                break;
            case "magic_missile":
                this.fx(ev.byYou ? "you" : "them", ev.ability, "fx-cast");
                break;
            default:
                this.fx(target, ev.ability, "fx-cast");
        }
    }

    // An icon that pops over a side's board and fades
    private fx(side: Side, ability: string, cls: string) {
        const wrap = this.popup?.q(side === "you" ? "#duelYouWrap" : "#duelOppWrap");
        if (!wrap) {
            return;
        }
        const img = document.createElement("img");
        img.className = "cast-fx " + cls;
        img.src = iconUrl("abilities/" + ability + ".png");
        img.alt = "";
        img.addEventListener("animationend", () => img.remove());
        setTimeout(() => img.remove(), 1500);
        wrap.appendChild(img);
    }

    // The side's board shakes
    private hit(side: Side) {
        const wrap = this.popup?.q(side === "you" ? "#duelYouWrap" : "#duelOppWrap");
        wrap?.classList.remove("hit");
        void wrap?.offsetWidth;
        wrap?.classList.add("hit");
    }

    // A missile flies across into the side's board
    private missileFx(side: Side) {
        const boards = this.popup?.q(".duel-boards");
        if (!boards) {
            return;
        }
        const img = document.createElement("img");
        img.className = "missile-fx " + (side === "you" ? "fly-left" : "fly-right");
        img.src = iconUrl("abilities/magic_missile.png");
        img.alt = "";
        img.addEventListener("animationend", () => {
            img.remove();
            this.hit(side);
        });
        setTimeout(() => img.remove(), 1500);
        boards.appendChild(img);
    }

    private feed(text: string, cls: string) {
        const list = this.popup?.q("#duelFeed");
        if (!list) {
            return;
        }
        const li = document.createElement("li");
        li.className = cls;
        li.textContent = text;
        list.prepend(li);
        while (list.childElementCount > FEED_LINES) {
            list.lastElementChild!.remove();
        }
    }

    public handleScry(s: DuelScry) {
        const popup = this.popup;
        if (!popup || this.ended) {
            return;
        }
        this.scried.set(s.letter, s.inWord);
        const chips = popup.q("#duelScry");
        chips.querySelector(`[data-letter="${s.letter}"]`)?.remove();
        const chip = document.createElement("span");
        chip.className = "scry-chip " + (s.inWord ? "tile-yellow" : "tile-grey");
        chip.dataset.letter = s.letter;
        chip.textContent = s.letter;
        chip.title = s.inWord ? `${s.letter} is in your word` : `${s.letter} isn't in your word`;
        chips.appendChild(chip);
        this.feed(chip.title, "mine");
    }

    public handleEyes(e: DuelEyes) {
        this.opponent?.showEyes(e.tiles);
    }

    public handleBoard(b: DuelBoard) {
        if (this.ended) {
            return;
        }
        if (b.yours) {
            this.board?.recolor(b.colors);
            this.game.toast("Reality shifted: your word changed");
        } else {
            this.opponent?.recolor(b.colors);
        }
    }

    // Reshape Reality: pick the opponent's new word
    public handleReshapeOptions(o: DuelReshapeOptions) {
        if (!this.popup || this.ended) {
            return;
        }
        if (o.words.length === 0) {
            this.game.toast("No words to reshape into");
            return;
        }
        const layer = this.openLayer("tpl-duel-reshape");
        layer.querySelector("#reshapeFor")!.textContent = this.opponentName;
        const list = layer.querySelector("#reshapeWords")!;
        for (const word of o.words) {
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "reshape-word";
            btn.textContent = word;
            btn.addEventListener("click", () => {
                this.closeLayer();
                this.game.send(ClientDuelReshape, { word });
            });
            list.appendChild(btn);
        }
        (list.firstElementChild as HTMLElement).focus();
    }

    // Trapped in a small Wordle until it's solved or failed
    public handleIllusionStart(s: IllusionStart) {
        const popup = this.popup;
        if (!popup || this.ended) {
            return;
        }
        this.closeLayer();
        this.opponent?.cancelPick();
        this.illusion?.layer.remove();
        const layer = popup.addLayer("tpl-duel-illusion");
        layer.querySelector("#illusionCaster")!.textContent = this.opponentName;
        const board = new WordleBoard(popup, layer.querySelector("#illusionBoard")!, layer.querySelector("#illusionSubmit")!, {
            wordLength: s.wordLength,
            rows: s.maxGuesses,
            idPrefix: "illusion-",
        });
        board.onSubmit = (guess) => this.game.send(ClientIllusionGuess, { guess });
        board.setKeymap(this.state?.you.keymap ?? "");
        this.board?.setDisabled(true);
        this.illusion = { board, layer };
        board.focus();
    }

    public handleIllusionGuess(res: WordleRes) {
        const board = this.illusion?.board;
        if (!board) {
            return;
        }
        if (res.blocked) {
            board.reject();
            this.game.toast("Your keyboard is stunned");
        } else if (!res.valid) {
            board.reject();
            this.game.toast("Not in word list");
        } else {
            board.colorRow(board.currentGuess - 1, res.colors);
        }
    }

    public handleIllusionEnd(end: IllusionEnd) {
        const ill = this.illusion;
        if (!ill) {
            return;
        }
        ill.board.lock();
        const status = ill.layer.querySelector("#illusionStatus")!;
        status.textContent = end.purified ? "The illusion fades away."
            : end.won ? "You saw through the illusion!" : `The word was ${end.solution}. You lost ${end.energyLost} energy.`;
        status.classList.add(end.won || end.purified ? "won" : "lost");
        this.illusion = null;
        setTimeout(() => {
            ill.layer.remove();
            this.board?.focus();
        }, ILLUSION_END_MS);
    }

    // Pickpocket: a yellow letter of theirs, revealed for good
    public handlePickpocket(p: DuelPickpocket) {
        this.opponent?.showPickpocket(p.row, p.col, p.letter);
        this.feed(`You pickpocketed a ${p.letter}`, "mine");
    }

    // Mend took a guess back, yours or theirs
    public handleGuessRemoved(r: DuelGuessRemoved) {
        if (this.ended) {
            return;
        }
        if (r.yours) {
            this.board?.removeGuess(r.row);
            if (this.out) {
                this.out = false;
                this.board?.unlock();
                this.setYouStatus(DEFAULT_HINT);
            }
        } else {
            this.opponent?.removeGuess(r.row);
            this.updateOpponentStatus();
        }
    }

    // Divine Intervention: the proclamation, and a clean slate if that's
    // what the gods chose. Everything else follows in other messages.
    public handleDivine(d: DuelDivine) {
        if (this.ended) {
            return;
        }
        proclaim(d.banner);
        this.feed(d.banner.charAt(0) + d.banner.slice(1).toLowerCase(), "divine");
        if (d.fate === FateCleanSlate) {
            this.out = false;
            this.board?.reset(this.start.maxGuesses);
            this.opponent?.reset(this.start.maxGuesses);
            this.popup?.q("#duelScry").replaceChildren();
            this.scried.clear();
            this.setYouStatus(DEFAULT_HINT);
            this.updateOpponentStatus();
        }
    }

    // The gods revealed a letter of your word and where it goes
    public handleReveal(r: DuelReveal) {
        const chips = this.popup?.q("#duelScry");
        if (!chips) {
            return;
        }
        const chip = document.createElement("span");
        chip.className = "scry-chip reveal-chip tile-green";
        chip.textContent = `${ORDINALS[r.col] ?? r.col + 1}: ${r.letter}`;
        chip.title = `The gods say your word's ${ORDINALS[r.col]} letter is ${r.letter}`;
        chips.appendChild(chip);
        this.feed(chip.title, "divine");
    }

    private updateOpponentStatus() {
        const opp = this.opponent;
        if (!opp || !this.popup) {
            return;
        }
        const status = this.popup.q("#duelOppStatus");
        status.textContent = opp.solved ? "Solved" : opp.done ? "Out of guesses" : `${opp.guesses}/${opp.rowCount}`;
        status.classList.toggle("solved", opp.solved);
        status.classList.toggle("out", opp.done && !opp.solved);
    }

    private setYouStatus(text: string) {
        this.popup?.q("#duelYouStatus").replaceChildren(text);
    }

    private setHint(text: string | null) {
        const hint = this.popup?.q("#duelTargetHint");
        if (hint) {
            hint.hidden = text === null;
            hint.textContent = text ?? "";
        }
    }

    public handleEnd(end: DuelEnd) {
        const popup = this.popup;
        if (!popup || this.ended) {
            return;
        }
        this.ended = true;
        this.stopwatch?.stop();
        this.typing.cancel();
        clearInterval(this.countdown);
        this.opponent?.cancelPick();
        this.closeLayer();
        this.illusion?.layer.remove();
        this.illusion = null;
        this.board?.lock();
        this.bar?.update(null);
        popup.q("#duelYouStun").hidden = true;
        popup.q("#duelOppStun").hidden = true;
        popup.q<HTMLButtonElement>("#duelForfeit").disabled = true;

        const layer = popup.addLayer("tpl-duel-result");
        const set = (id: string, text: string) => layer.querySelector("#" + id)!.textContent = text;
        const name = this.opponentName;
        const title = end.outcome == DuelWin ? "Victory" : end.outcome == DuelDraw ? "Draw" : "Defeat";
        layer.querySelector(".result-panel")!.classList.add("duel-" + title.toLowerCase());
        set("duelResultTitle", title);

        let text: string;
        if (end.reason == DuelForfeit) {
            text = end.outcome == DuelWin ? `${name} gave up.` : `You gave up against ${name}.`;
        } else if (end.reason == DuelDisconnect) {
            text = end.outcome == DuelWin ? `${name} left the duel.` : "You left the duel.";
        } else if (end.reason == DuelOutOfGuesses) {
            text = `Neither of you found the word.`;
        } else if (end.reason == DuelTimeUp) {
            text = "Time ran out. The gods call it a draw.";
        } else {
            text = end.outcome == DuelWin ? `You found the word before ${name}.` : `${name} found the word first.`;
        }
        set("duelResultText", text);
        // Green if someone found it
        renderSolution(layer.querySelector<HTMLDivElement>("#duelSolution")!, end.solution, end.reason == DuelSolved);
        if (end.ranked) {
            this.showEloChange(layer, end);
        }
    }

    // The ranked part of the result: points won or lost, rank before and
    // after, and why it moved that much
    private showEloChange(layer: HTMLElement, end: DuelEnd) {
        const q = (id: string) => layer.querySelector<HTMLElement>("#" + id)!;
        const ladder = this.game.state.ladder;
        const delta = end.eloAfter - end.eloBefore;
        q("duelElo").hidden = false;
        const change = q("duelEloChange");
        change.textContent = `${signed(delta)} elo`;
        change.classList.add(delta > 0 ? "up" : delta < 0 ? "down" : "even");

        const before = tierFor(end.eloBefore, ladder);
        const after = tierFor(end.eloAfter, ladder);
        q("duelRankBefore").replaceWith(Object.assign(rankBadge(before, end.eloBefore), { id: "duelRankBefore" }));
        q("duelRankAfter").replaceWith(Object.assign(rankBadge(after, end.eloAfter), { id: "duelRankAfter" }));
        const promo = q("duelPromo");
        if (before && after && before.id !== after.id) {
            const up = after.minElo > before.minElo;
            promo.textContent = up ? `Promoted to ${after.name}` : `Demoted to ${after.name}`;
            promo.classList.add(up ? "up" : "down", "rank-" + after.family);
        }
        const margin = end.outcome == DuelDraw ? "" : ` · margin ×${end.margin.toFixed(2)}`;
        q("duelBreakdown").textContent = `You were expected to win ${Math.round(end.expected * 100)}%${margin}`;
    }

    // The connection dropped: the server has already counted it as a loss
    public abandon() {
        this.popup?.close();
        if (!this.ended) {
            this.game.toast("Your duel ended when you lost connection");
        }
    }
}
