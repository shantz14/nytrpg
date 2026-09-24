import { RankTier } from "./protocol.gen.js";

// Text color for each rank family, on the canvas. The CSS has the same colors
// as .rank-<family> classes.
export const RANK_COLORS: {[family: string]: string} = {
    iron: "#8b9099",
    bronze: "#cf9160",
    silver: "#c9d1db",
    gold: "#e8c872",
    diamond: "#7fd6f7",
    master: "#ff7a59",
};

// The rank an elo is in. ladder is lowest first, from the welcome.
export function tierFor(elo: number, ladder: RankTier[]): RankTier | null {
    let tier: RankTier | null = null;
    for (const t of ladder) {
        if (elo >= t.minElo) {
            tier = t;
        }
    }
    return tier ?? ladder[0] ?? null;
}

// The rank above the one elo is in, null at the top
export function nextTier(elo: number, ladder: RankTier[]): RankTier | null {
    return ladder.find((t) => t.minElo > elo) ?? null;
}

// How far through its rank elo is, 0 to 1. 1 at the top of the ladder.
export function tierProgress(elo: number, ladder: RankTier[]): number {
    const tier = tierFor(elo, ladder);
    const next = nextTier(elo, ladder);
    if (!tier || !next) {
        return 1;
    }
    // Iron 3 runs from 0, but show it as 100 wide like the rest
    const start = Math.max(tier.minElo, next.minElo - 100);
    return Math.min(1, Math.max(0, (elo - start) / (next.minElo - start)));
}

// "+12" or "−14" (a real minus sign)
export function signed(n: number): string {
    return n > 0 ? `+${n}` : n < 0 ? `−${-n}` : "0";
}
