import { ChatMsg, ClassInfo } from "./protocol.gen.js";
import { className } from "./classes.js";

// Lines kept in the log, the oldest go first
export const MAX_LOG_LINES = 100;
// How long the log stays bright after a new message
const ACTIVE_MS = 6000;

// What one log line shows: "Merlin alice: hello"
export type ChatLine = {
    // Character name and class ID, empty for speakers without a character
    char: string;
    cls: string;
    user: string;
    text: string;
    // We said it
    self: boolean;
};

export function chatLine(msg: ChatMsg, selfId: number): ChatLine {
    return {
        char: msg.char ?? "",
        cls: msg.class ?? "",
        user: msg.name ?? "",
        text: msg.msg,
        self: msg.id === selfId,
    };
}

// Splits text into lines no wider than maxWidth (as measured), breaking words
// that don't fit on their own. More than maxLines ends in an ellipsis.
export function wrapText(text: string, maxWidth: number, measure: (s: string) => number, maxLines = 3): string[] {
    const lines: string[] = [];
    let line = "";
    const push = (s: string) => {
        lines.push(s);
        line = "";
    };
    for (const word of text.split(/\s+/).filter(Boolean)) {
        const joined = line ? line + " " + word : word;
        if (measure(joined) <= maxWidth) {
            line = joined;
            continue;
        }
        if (line) {
            push(line);
        }
        // A word wider than a whole line gets cut wherever it has to be
        let rest = word;
        while (measure(rest) > maxWidth && rest.length > 1) {
            let n = rest.length - 1;
            while (n > 1 && measure(rest.slice(0, n)) > maxWidth) {
                n--;
            }
            push(rest.slice(0, n));
            rest = rest.slice(n);
        }
        line = rest;
    }
    if (line) {
        lines.push(line);
    }
    if (lines.length <= maxLines) {
        return lines;
    }
    const kept = lines.slice(0, maxLines);
    let last = kept[maxLines - 1];
    while (last && measure(last + "…") > maxWidth) {
        last = last.slice(0, -1);
    }
    kept[maxLines - 1] = last.trimEnd() + "…";
    return kept;
}

// The scrolling chat history above the chat box. Kept for this session only,
// it carries on across reconnects.
export class ChatLog {
    el: HTMLElement;
    private activeTimer: ReturnType<typeof setTimeout> | null;

    constructor(el: HTMLElement) {
        this.el = el;
        this.activeTimer = null;
    }

    public add(msg: ChatMsg, selfId: number, classes: ClassInfo[]) {
        const line = chatLine(msg, selfId);
        const doc = this.el.ownerDocument;
        // Only follow new messages if the player hasn't scrolled up to read
        const atBottom = this.el.scrollHeight - this.el.scrollTop - this.el.clientHeight < 8;

        const row = doc.createElement("div");
        row.className = line.self ? "chat-line self" : "chat-line";
        // textContent only: chat is whatever other players typed
        if (line.char) {
            const char = doc.createElement("span");
            char.className = "chat-char class-" + line.cls;
            char.textContent = line.char;
            char.title = className(classes, line.cls);
            row.appendChild(char);
        }
        const user = doc.createElement("span");
        user.className = line.char ? "chat-user" : "chat-user solo";
        user.textContent = line.user;
        const text = doc.createElement("span");
        text.className = "chat-text";
        text.textContent = line.text;
        row.append(user, text);

        this.el.appendChild(row);
        while (this.el.childElementCount > MAX_LOG_LINES) {
            this.el.firstElementChild!.remove();
        }
        if (atBottom) {
            this.el.scrollTop = this.el.scrollHeight;
        }

        this.el.classList.add("active");
        if (this.activeTimer) {
            clearTimeout(this.activeTimer);
        }
        this.activeTimer = setTimeout(() => this.el.classList.remove("active"), ACTIVE_MS);
    }
}
