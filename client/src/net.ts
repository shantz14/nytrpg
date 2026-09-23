import { ClientMsg, ServerMsg } from "./protocol.gen.js";

declare const MessagePack: typeof import("@msgpack/msgpack");

type Handler = (data: any) => void;

// The websocket to the server. Every message is a msgpack array [type, payload],
// features register a handler for each message type they care about.
export class Connection {
    sock: WebSocket;
    handlers: Map<ServerMsg, Handler>;

    constructor(url: string) {
        this.handlers = new Map();
        this.sock = new WebSocket(url);
        this.sock.binaryType = "arraybuffer";
        this.sock.onmessage = (e) => this.receive(e.data as ArrayBuffer);
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
        if (this.sock.readyState !== WebSocket.OPEN) {
            return false;
        }
        this.sock.send(MessagePack.encode([type, data]));
        return true;
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
