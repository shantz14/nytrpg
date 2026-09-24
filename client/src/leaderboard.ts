import { InputDriver } from "./input-driver.js";
import { UserData } from "./login.js";
import { Popup } from "./popup.js";
import { formatTime } from "./wordle.js";
import { charLabel } from "./classes.js";
import { ClassInfo } from "./protocol.gen.js";

const URL = "/leaderboard";

type Row = {
    place: number,
    uname: string,
    characterId: number,
    char: string,
    class: string,
    guesses: number,
    time: number
}

type LeaderboardRes = {
    date: string,
    today: string,
    page: number,
    pageSize: number,
    total: number,
    rows: Array<Row>
}

// Shift a YYYY-MM-DD date by some days
export function shiftDate(date: string, days: number): string {
    const d = new Date(date + "T00:00:00Z");
    d.setUTCDate(d.getUTCDate() + days);
    return d.toISOString().slice(0, 10);
}

// "2026-09-23" to "Wed, Sep 23, 2026"
export function formatDate(date: string): string {
    const d = new Date(date + "T00:00:00Z");
    if (isNaN(d.getTime())) {
        return date;
    }
    return d.toLocaleDateString("en-US", { weekday: "short", month: "short", day: "numeric", year: "numeric", timeZone: "UTC" });
}

export class Leaderboard {
    userData: UserData;
    // The character being played, its rows are highlighted
    characterId: number;
    classes: ClassInfo[];
    inputDriver: InputDriver;
    // Empty means let the server pick today
    date: string;
    page: number;
    last: LeaderboardRes | null;
    popup: Popup | null;

    constructor(userData: UserData, characterId: number, classes: ClassInfo[], inputDriver: InputDriver) {
        this.userData = userData;
        this.characterId = characterId;
        this.classes = classes;
        this.inputDriver = inputDriver;
        this.date = "";
        this.page = 0;
        this.last = null;
        this.popup = null;
    }

    public run() {
        const popup = Popup.open("tpl-leaderboard", this.inputDriver);
        if (!popup) {
            return;
        }
        this.popup = popup;
        popup.on(popup.q("#pageUp"), "click", () => this.pageUp());
        popup.on(popup.q("#pageDown"), "click", () => this.pageDown());
        popup.on(popup.q("#nextDay"), "click", () => this.nextDay());
        popup.on(popup.q("#prevDay"), "click", () => this.prevDay());
        this.populate();
    }

    private async populate() {
        this.popup?.q("#lbWrap").classList.add("loading");
        const params = new URLSearchParams({ date: this.date, page: String(this.page) });
        const data: LeaderboardRes | null = await fetch(URL + "?" + params.toString())
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error getting leadboard data. Status: ${response.status}`);
            }
            return response.json();
        })
        .catch(error => {
            console.error('Error parsing leaderboard data:', error);
            return null;
        });

        if (!this.popup) {
            return;
        }
        this.popup.q("#lbWrap").classList.remove("loading");
        if (!data) {
            return;
        }
        const body = this.popup.q<HTMLTableSectionElement>("#lbBody");
        this.last = data;
        this.date = data.date;
        this.page = data.page;

        this.popup.q("#date").textContent = formatDate(data.date);
        this.popup.q("#dateTag").textContent = data.date == data.today ? "Today" : "";

        body.replaceChildren();
        if (data.rows.length == 0) {
            const tr = body.insertRow();
            const td = tr.insertCell();
            td.colSpan = 4;
            td.className = "empty";
            td.textContent = "No results for this day.";
        }
        const columns = ["col-place", "col-name", "col-num", "col-num"];
        for (const row of data.rows) {
            const tr = body.insertRow();
            if (row.place <= 3) {
                tr.classList.add("place-" + row.place);
            }
            if (row.characterId == this.characterId) {
                tr.classList.add("me");
            }
            // textContent, never innerHTML: usernames come from users
            const cells = [String(row.place), row.uname, String(row.guesses), formatTime(row.time)];
            cells.forEach((text, i) => {
                const td = tr.insertCell();
                td.className = columns[i];
                td.textContent = text;
            });
            // The character under the username
            const char = document.createElement("span");
            char.className = "lb-char";
            char.textContent = charLabel(this.classes, row.char, row.class);
            tr.cells[1].appendChild(char);
        }

        const pages = this.lastPage() + 1;
        this.popup.q("#pageInfo").textContent = "Page " + (this.page + 1) + " of " + pages;
        this.popup.q("#pager").classList.toggle("hidden", pages <= 1);

        this.updateButtons();
    }

    private lastPage(): number {
        if (!this.last || this.last.total == 0) {
            return 0;
        }
        return Math.ceil(this.last.total / this.last.pageSize) - 1;
    }

    private updateButtons() {
        const set = (id: string, disabled: boolean) => {
            this.popup!.q<HTMLButtonElement>("#" + id).disabled = disabled;
        };
        set("pageUp", this.page <= 0);
        set("pageDown", this.page >= this.lastPage());
        set("nextDay", !this.last || this.date >= this.last.today);
    }

    private nextDay() {
        if (!this.last || this.date >= this.last.today) {
            return;
        }
        this.date = shiftDate(this.date, 1);
        this.page = 0;
        this.populate();
    }

    private prevDay() {
        if (!this.date) {
            return;
        }
        this.date = shiftDate(this.date, -1);
        this.page = 0;
        this.populate();
    }

    private pageUp() {
        if (this.page <= 0) {
            return;
        }
        this.page--;
        this.populate();
    }

    private pageDown() {
        if (this.page >= this.lastPage()) {
            return;
        }
        this.page++;
        this.populate();
    }

}
