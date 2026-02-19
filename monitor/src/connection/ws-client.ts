import WebSocket from "ws";
import type {
  Envelope,
  CommandResultPayload,
} from "./protocol.js";
import { MSG } from "./protocol.js";

type MessageHandler = (env: Envelope) => void;

export interface WSClientOptions {
  url: string;
  onMessage: MessageHandler;
  onConnect?: () => void;
  onDisconnect?: () => void;
}

export class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private onMessage: MessageHandler;
  private onConnect?: () => void;
  private onDisconnect?: () => void;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private shouldReconnect = true;
  private commandCallbacks = new Map<
    string,
    (result: CommandResultPayload) => void
  >();
  private cmdCounter = 0;

  constructor(opts: WSClientOptions) {
    this.url = opts.url;
    this.onMessage = opts.onMessage;
    this.onConnect = opts.onConnect;
    this.onDisconnect = opts.onDisconnect;
  }

  connect(): void {
    if (this.ws) return;

    try {
      this.ws = new WebSocket(this.url);

      this.ws.on("open", () => {
        this.reconnectDelay = 1000;
        this.onConnect?.();
      });

      this.ws.on("message", (data: WebSocket.Data) => {
        try {
          // Handle batched messages (newline-separated)
          const raw = data.toString();
          const messages = raw.split("\n").filter((m) => m.trim());
          for (const msg of messages) {
            const env: Envelope = JSON.parse(msg);

            // Handle command results internally
            if (env.type === MSG.COMMAND_RESULT) {
              const result = env.payload as CommandResultPayload;
              const cb = this.commandCallbacks.get(result.id);
              if (cb) {
                cb(result);
                this.commandCallbacks.delete(result.id);
              }
            }

            this.onMessage(env);
          }
        } catch {
          // Ignore parse errors
        }
      });

      this.ws.on("close", () => {
        this.ws = null;
        this.onDisconnect?.();
        this.scheduleReconnect();
      });

      this.ws.on("error", () => {
        this.ws?.close();
      });
    } catch {
      this.scheduleReconnect();
    }
  }

  disconnect(): void {
    this.shouldReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  subscribe(channel: string): void {
    this.send({ type: MSG.SUBSCRIBE, payload: { channel } });
  }

  unsubscribe(channel: string): void {
    this.send({ type: MSG.UNSUBSCRIBE, payload: { channel } });
  }

  command(
    action: string,
    args: Record<string, unknown>
  ): Promise<CommandResultPayload> {
    const id = `cmd-${++this.cmdCounter}-${Date.now()}`;
    return new Promise((resolve) => {
      this.commandCallbacks.set(id, resolve);
      this.send({ type: MSG.COMMAND, payload: { id, action, args } });

      // Timeout after 30 seconds
      setTimeout(() => {
        if (this.commandCallbacks.has(id)) {
          this.commandCallbacks.delete(id);
          resolve({
            id,
            success: false,
            error: "command timed out",
          });
        }
      }, 30000);
    });
  }

  private send(env: Envelope): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(env));
    }
  }

  private scheduleReconnect(): void {
    if (!this.shouldReconnect) return;
    if (this.reconnectTimer) return;

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.reconnectDelay);

    this.reconnectDelay = Math.min(
      this.reconnectDelay * 2,
      this.maxReconnectDelay
    );
  }
}
