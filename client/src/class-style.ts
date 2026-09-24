// How each class's name looks, on the canvas and in the DOM. The DOM side is
// the .class-<id> rules in styles.css (--class-color, --class-font), a test
// keeps the two in sync.
export type ClassStyle = {
    color: string;
    // CSS font family, loaded from Google Fonts in index.html
    font: string;
    weight: number;
};

export const CLASS_STYLE: Record<string, ClassStyle> = {
    // Steel, and the carved Roman capitals of a knight's order
    knight: { color: "#c9ced6", font: "Cinzel", weight: 700 },
    // Arcane violet, a spellbook's uncial script
    wizard: { color: "#9d8cf2", font: "Uncial Antiqua", weight: 400 },
    // Poison green, worn old printed caps off a wanted poster
    rogue: { color: "#6fcf8e", font: "IM Fell English SC", weight: 400 },
    // Holy gold, an illuminated manuscript's serif
    cleric: { color: "#e8c872", font: "Cormorant Garamond", weight: 700 },
};

// Classes the client doesn't know yet look like the rest of the UI
const DEFAULT_STYLE: ClassStyle = { color: "#d4d4d4", font: "IBM Plex Mono", weight: 600 };

export function classStyle(id: string): ClassStyle {
    return CLASS_STYLE[id] ?? DEFAULT_STYLE;
}

// A canvas font string for a class's name at size px
export function classFont(id: string, size: number): string {
    const s = classStyle(id);
    return `${s.weight} ${size}px "${s.font}", serif`;
}

// Asks the browser to download every class font now. Canvas text doesn't
// trigger loading on its own, it silently falls back until the font is ready.
export function loadClassFonts() {
    if (typeof document === "undefined" || !document.fonts) {
        return;
    }
    for (const id in CLASS_STYLE) {
        document.fonts.load(classFont(id, 16)).catch(() => { /* fallback font is fine */ });
    }
}
