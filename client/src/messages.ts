import { PlayerState } from "./game-objects.js";
import { WordleColor, WordleStatus } from "./wordle.js";

export type ServerUpdateType = number;

export const ServerUpdatePos: ServerUpdateType = 1;
export const ServerWordleResponse: ServerUpdateType = 2;
export const ServerWordleResult: ServerUpdateType = 3;

export type ServerUpdate = {
    updateType: ServerUpdateType;
    data: Uint8Array;
}

// Players positions and a player to unregister
export type UpdateState = {
    players: {[key: string]: PlayerState};
    unregister: number;
}

// Win/loss/inGame and array of colors for da letters
export type WordleResponse = {
    valid: boolean;
    status: WordleStatus;
    colors: Array<WordleColor>;
    solution: string;
}



export type ClientUpdateType = number;

export const ClientUpdatePos: ClientUpdateType = 1;
export const ClientSendWordle: ClientUpdateType = 2;

export type ClientUpdate = {
    updateType: ClientUpdateType;
    data: Uint8Array;
}

export type WordleReq = {
    id: number;
    guess: string;
    guessCount: number;
    time: number;
}





