import { PlayerState } from "./game-objects";
import { WordleColor, WordleStatus } from "./wordle";

export type ServerUpdateType = number;

export const ServerUpdatePos: ServerUpdateType = 1;
export const ServerWordleRes: ServerUpdateType = 2;

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
export type WordleRes = {
    status: WordleStatus;
    colors: Array<WordleColor>;
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
}




