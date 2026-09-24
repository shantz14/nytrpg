// Sends a changing value at most once per interval, always ending on the latest
// one. The first change goes out at once. Changes that come faster are held and
// only the newest is sent when the interval is up. Repeats of the last sent
// value are skipped.
export class LatestThrottle<T> {
    private last: T | undefined;
    private lastAt: number;
    private pending: { value: T } | null;
    private timer: ReturnType<typeof setTimeout> | null;

    constructor(private send: (value: T) => void, private intervalMs: number, private now: () => number = () => performance.now()) {
        this.last = undefined;
        this.lastAt = -Infinity;
        this.pending = null;
        this.timer = null;
    }

    public push(value: T) {
        if (this.timer !== null) {
            this.pending = { value };
            return;
        }
        if (value === this.last) {
            return;
        }
        const wait = this.lastAt + this.intervalMs - this.now();
        if (wait <= 0) {
            this.fire(value);
            return;
        }
        this.pending = { value };
        this.timer = setTimeout(() => this.flush(), wait);
    }

    // Drops anything waiting to go out
    public cancel() {
        if (this.timer !== null) {
            clearTimeout(this.timer);
        }
        this.timer = null;
        this.pending = null;
    }

    private flush() {
        this.timer = null;
        const p = this.pending;
        this.pending = null;
        if (p && p.value !== this.last) {
            this.fire(p.value);
        }
    }

    private fire(value: T) {
        this.last = value;
        this.lastAt = this.now();
        this.send(value);
    }
}
