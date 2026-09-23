import { ClientMsg, ServerMsg } from "./protocol.gen.js";

declare const MessagePack: typeof import("@msgpack/msgpack");

type Handler = (data: any) => void;

const MIN_RETRY_MS = 500;
const MAX_RETRY_MS = 10000;
// Server close code: this player connected from somewhere else, don't fight it
const CLOSE_REPLACED = 4001;

// The websocket to the server. Every message is a msgpack array [type, payload],
// features register a handler for each message type they care about.
//
// If the connection drops it reconnects with backoff. Handlers stay registered,
// and the server sends a fresh welcome on every connect.
export class Connection {
    url: string;
    sock: WebSocket | null;
    handlers: Map<ServerMsg, Handler>;
    retryMs: number;
    stopped: boolean;
    // Called when the connection is lost and when it's back
    onDisconnect: (() => void) | null;
    onReconnect: (() => void) | null;
    // Called instead of reconnecting when we logged in somewhere else
    onReplaced: (() => void) | null;
    private everConnected: boolean;

    constructor(url: string) {
        this.url = url;
        this.sock = null;
        this.handlers = new Map();
        this.retryMs = MIN_RETRY_MS;
        this.stopped = false;
        this.onDisconnect = null;
        this.onReconnect = null;
        this.onReplaced = null;
        this.everConnected = false;
        this.connect();
    }

    // Registers the handler for one message type, T is the payload type
    public on<T>(type: ServerMsg, handler: (data: T) => void) {
        if (this.handlers.has(type)) {
            throw new Error(`Handler for message ${type} already registered`);
        }
        this.handlers.set(type, handler);
    }

    // Returns false if the socket isn't open and nothing was sent
    public send(type: ClientMsg, data: unknown): boolean {
        if (!this.sock || this.sock.readyState !== WebSocket.OPEN) {
            return false;
        }
        this.sock.send(MessagePack.encode([type, data]));
        return true;
    }

    // Closes for good, no reconnecting
    public close() {
        this.stopped = true;
        this.sock?.close();
    }

    private connect() {
        const sock = new WebSocket(this.url);
        this.sock = sock;
        sock.binaryType = "arraybuffer";
        sock.onmessage = (e) => this.receive(e.data as ArrayBuffer);
        sock.onopen = () => {
            this.retryMs = MIN_RETRY_MS;
            if (this.everConnected) {
                this.onReconnect?.();
            }
            this.everConnected = true;
        };
        sock.onclose = (e) => {
            if (this.sock !== sock || this.stopped) {
                return;
            }
            if (e.code === CLOSE_REPLACED) {
                this.stopped = true;
                this.onReplaced?.();
                return;
            }
            // Only report the drop once per outage, not on every failed retry
            if (this.retryMs === MIN_RETRY_MS) {
                this.onDisconnect?.();
            }
            setTimeout(() => this.connect(), this.retryMs);
            this.retryMs = Math.min(this.retryMs * 2, MAX_RETRY_MS);
        };
    }

    private receive(raw: ArrayBuffer) {
        const [type, data] = MessagePack.decode(new Uint8Array(raw)) as [ServerMsg, unknown];
        const handler = this.handlers.get(type);
        if (handler) {
            handler(data);
        } else {
            console.warn("No handler for server message", type);
        }
    }
}
