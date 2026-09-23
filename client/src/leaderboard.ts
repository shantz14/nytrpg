import { InputDriver } from "./input-driver.js";
import { UserData } from "./login.js";
import { formatTime } from "./wordle.js";

const URL = "/leaderboard";

type Row = {
    place: number,
    uname: string,
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
function shiftDate(date: string, days: number): string {
    const d = new Date(date + "T00:00:00Z");
    d.setUTCDate(d.getUTCDate() + days);
    return d.toISOString().slice(0, 10);
}

export class Leaderboard {
    userData: UserData;
    inputDriver: InputDriver;
    // Empty means let the server pick today
    date: string;
    page: number;
    last: LeaderboardRes | null;

    constructor(userData: UserData, inputDriver: InputDriver) {
        this.userData = userData;
        this.inputDriver = inputDriver;
        this.date = "";
        this.page = 0;
        this.last = null;
    }

    public run() {
        this.inputDriver.setPopupFocused();
        this.createPopup();
        this.populate();
    }

    private async populate() {
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

        const body = document.getElementById("lbBody") as HTMLTableSectionElement | null;
        if (!data || !body) {
            return;
        }
        this.last = data;
        this.date = data.date;
        this.page = data.page;

        const dateText = document.getElementById("date") as HTMLHeadingElement;
        dateText.textContent = data.date;

        body.replaceChildren();
        if (data.rows.length == 0) {
            const tr = body.insertRow();
            const td = tr.insertCell();
            td.colSpan = 4;
            td.textContent = "No records for this day...";
        }
        for (const row of data.rows) {
            const tr = body.insertRow();
            // textContent, never innerHTML: usernames come from users
            for (const text of [String(row.place), row.uname, String(row.guesses), formatTime(row.time)]) {
                tr.insertCell().textContent = text;
            }
        }

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
            const button = document.getElementById(id) as HTMLButtonElement | null;
            if (button) {
                button.disabled = disabled;
            }
        };
        set("pageUp", this.page <= 0);
        set("pageDown", this.page >= this.lastPage());
        set("nextDay", !this.last || this.date >= this.last.today);
    }

    private createPopup() {
        const tpl = document.getElementById("tpl-leaderboard") as HTMLTemplateElement;
        document.getElementById("container")!.appendChild(tpl.content.cloneNode(true));

        document.getElementById("pageUp")?.addEventListener("click", () => this.pageUp());
        document.getElementById("pageDown")?.addEventListener("click", () => this.pageDown());
        document.getElementById("nextDay")?.addEventListener("click", () => this.nextDay());
        document.getElementById("prevDay")?.addEventListener("click", () => this.prevDay());
        const exit = document.getElementById("exit");
        exit?.addEventListener("click", () => {
            this.inputDriver.setGameFocused();
            const popup = document.getElementById("leaderboardPopup") as HTMLDivElement;
            popup.remove();
        });
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
