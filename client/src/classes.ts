import { ClassInfo } from "./protocol.gen.js";

// A class's display name, or its id if the client doesn't know it
export function className(classes: ClassInfo[], id: string): string {
    return classes.find((c) => c.id === id)?.name ?? id;
}

// "Merlin (Wizard)", the line under a player's username. Empty for entities
// that aren't characters.
export function charLabel(classes: ClassInfo[], char: string, classId: string): string {
    if (!char) {
        return "";
    }
    return `${char} (${className(classes, classId)})`;
}
