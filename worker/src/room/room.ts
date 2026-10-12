// The Room Durable Object: one instance per room code, holding the room's
// WebSockets and its state. Workers-only: everything here is runtime, which
// is exactly why the state machine it drives lives in room/state.ts.
//
// All room logic is a pure function. This file is thin glue only: it loads
// the state, turns a WebSocket event into an Event, calls handle, saves the
// new state, and carries out the effects in order. Every rule in Appendix A
// is tested through handle in plain Node; a rule that appears here is a
// mistake to move there and a test to write against it.

import { DurableObject } from "cloudflare:workers";
import { parseClient, isTooOld } from "../client.ts";
import { freshKey, sha256hex } from "../codes.ts";
import { pickAnswer } from "../words.ts";
import type { Mode, ServerMessage } from "./protocol.ts";
import { parseClientMessage } from "./protocol.ts";
import type { Event, Effect, RoomState } from "./state.ts";
import { createRoom, handle, nextDeadline } from "./state.ts";

// A socket's memory of itself. seat is 0 until a hello is accepted; helloBy
// is the deadline for that hello, 0 once attached; recent holds this socket's
// last frames for the rate limit, and survives hibernation with it.
interface Attachment {
  seat: number;
  helloBy: number;
  recent: number[];
}

const HELLO_MS = 10_000;

export class Room extends DurableObject<Env> {
  state: RoomState | null = null;

  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
    // The heartbeat answers itself at the edge, without waking the room.
    ctx.setWebSocketAutoResponse(new WebSocketRequestResponsePair("ping", "pong"));
    ctx.blockConcurrencyWhile(async () => {
      this.state = await ctx.storage.get<RoomState>("room") ?? null;
    });
  }

  async fetch(request: Request): Promise<Response> {
    const pathname = new URL(request.url).pathname;

    // POST /init makes the room. rooms.ts validates; this trusts it.
    if (request.method === "POST" && pathname === "/init") {
      if (this.state !== null) return json(409, { error: "taken" });
      const init = await request.json() as InitBody;
      const now = Date.now();
      const fresh = await freshKey();
      const state = createRoom(
        { code: init.code, mode: init.mode as Mode, length: init.length, autoStart: init.autoStart },
        { now, connected: new Set<number>(), pickAnswer },
        { keyHash: fresh.keyHash },
      );
      if (state.mode === "async") {
        // The room row and any results a dead same-code room left behind are
        // one transaction: if a live row already owns the code, nothing is
        // written and the caller draws another code.
        try {
          await this.env.DB.batch([
            this.env.DB
              .prepare("DELETE FROM async_results WHERE code = ?1 AND code IN (SELECT code FROM async_rooms WHERE code = ?1 AND expires_at <= ?2)")
              .bind(init.code, now),
            this.env.DB
              .prepare("DELETE FROM async_rooms WHERE code = ?1 AND expires_at <= ?2")
              .bind(init.code, now),
            this.env.DB
              .prepare("INSERT INTO async_rooms (code, length, answer, created_at, expires_at) VALUES (?1, ?2, ?3, ?4, ?5)")
              .bind(init.code, init.length, state.answer, now, state.expiresAt),
          ]);
        } catch {
          return json(409, { error: "taken" });
        }
      }
      this.state = state;
      await this.ctx.storage.put("room", this.state);
      await this.schedule();
      return json(201, { key: fresh.key });
    }

    if ((request.headers.get("Upgrade") ?? "").toLowerCase() === "websocket") {
      const pair = new WebSocketPair();
      const [client, server] = Object.values(pair);
      this.ctx.acceptWebSocket(server);
      server.serializeAttachment(
        { seat: 0, helloBy: Date.now() + HELLO_MS, recent: [] } satisfies Attachment,
      );
      await this.schedule();
      return new Response(null, { status: 101, webSocket: client });
    }

    return json(404, { error: "not_found" });
  }

  async webSocketMessage(ws: WebSocket, message: string | ArrayBuffer): Promise<void> {
    if (message === "ping") {
      // The auto-response answered this at the edge; the room hears nothing.
      return;
    }
    if (typeof message !== "string" || message.length > 4096) {
      closeSocket(ws, 4001, "bad frame");
      return;
    }

    // Rate limit, Appendix A.3: 20 frames in any 10 seconds, ping included.
    const attachment = ws.deserializeAttachment() as Attachment;
    const now = Date.now();
    attachment.recent = attachment.recent.filter((t) => t > now - 10_000);
    attachment.recent.push(now);
    if (attachment.recent.length > 20) {
      closeSocket(ws, 4008, "too many messages");
      return;
    }
    ws.serializeAttachment(attachment);

    const parsed = parseClientMessage(message);
    if (!parsed.ok) {
      if (parsed.reason === "not_json") {
        closeSocket(ws, 4001, "bad frame");
      } else {
        this.trySend(ws, { t: "error", reason: parsed.reason === "unknown_type" ? "unknown_type" : "bad_message" });
      }
      return;
    }

    let event: Event;
    if (attachment.seat === 0) {
      // The hello is the first message, and the only one the socket may send.
      if (parsed.msg.t !== "hello") {
        closeSocket(ws, 4001, "hello first");
        return;
      }
      const hello = parsed.msg;
      const client = parseClient(hello.client);
      if (hello.v !== 1 || client === null || isTooOld(client.version, this.env.MIN_CLIENT)) {
        this.trySend(ws, { t: "gone", message: this.env.GONE_MESSAGE });
        closeSocket(ws, 4003, "gone");
        return;
      }
      // The key proves a seat; the fresh key arms a new one.
      const keyHash = hello.key === "" ? null : await sha256hex(hello.key);
      const fresh = await freshKey();
      event = { type: "hello", msg: hello, keyHash, fresh };
    } else {
      if (parsed.msg.t === "hello") {
        this.trySend(ws, { t: "error", reason: "not_allowed" });
        return;
      }
      event = { type: "message", seat: attachment.seat, msg: parsed.msg };
    }

    await this.runEvent(event, ws);
  }

  async webSocketClose(ws: WebSocket, _code: number, _reason: string, _wasClean: boolean): Promise<void> {
    await this.socketGone(ws);
  }

  async webSocketError(ws: WebSocket, _error: unknown): Promise<void> {
    await this.socketGone(ws);
  }

  async alarm(): Promise<void> {
    // First the sockets that never said hello: their deadline has passed.
    const now = Date.now();
    for (const ws of this.ctx.getWebSockets()) {
      const attachment = attachmentOf(ws);
      if (attachment && attachment.seat === 0 && attachment.helloBy > 0 && attachment.helloBy <= now) {
        closeSocket(ws, 4001, "no hello");
      }
    }
    await this.runEvent({ type: "alarm" }, null);
  }

  // socketGone runs the disconnect event when the last socket of a seat has
  // gone, then makes sure the socket itself is closed.
  private async socketGone(ws: WebSocket): Promise<void> {
    const attachment = attachmentOf(ws);
    if (attachment && attachment.seat !== 0) {
      let others = false;
      for (const candidate of this.ctx.getWebSockets()) {
        if (candidate === ws) continue;
        const ours = attachmentOf(candidate);
        if (ours && ours.seat === attachment.seat) {
          others = true;
          break;
        }
      }
      if (!others) await this.runEvent({ type: "disconnect", seat: attachment.seat }, ws);
    }
    closeSocket(ws, 1000, "closing");
  }

  // runEvent is the one path an event takes: measure who is connected, ask
  // the state machine, carry out the effects, save, and re-arm the clock.
  private async runEvent(event: Event, socket: WebSocket | null): Promise<void> {
    const connected = new Set<number>();
    for (const ws of this.ctx.getWebSockets()) {
      if (event.type === "disconnect" && ws === socket) continue;
      const attachment = attachmentOf(ws);
      if (attachment && attachment.seat !== 0) connected.add(attachment.seat);
    }

    const { state, effects } = handle(this.state, event, {
      now: Date.now(),
      connected,
      pickAnswer,
    });

    // The new state is committed before any effect runs. An effect can await
    // (the D1 write of a finished async round does), and a Durable Object
    // takes its next event while a non-storage await is in flight: that event
    // must see this one's state, or one of the two updates is lost. The
    // storage write also holds the outgoing messages below until it is
    // durable (the output gate), so no player hears of a change the room
    // could still forget.
    this.state = state;
    if (this.state !== null) {
      await this.ctx.storage.put("room", this.state);
    }
    for (const effect of effects) {
      await this.apply(effect, socket);
    }
    await this.schedule();
  }

  private async apply(effect: Effect, socket: WebSocket | null): Promise<void> {
    switch (effect.kind) {
      case "reply":
        if (socket !== null) this.trySend(socket, effect.msg);
        return;

      case "attach": {
        if (socket === null) return;
        const previous = attachmentOf(socket);
        socket.serializeAttachment(
          { seat: effect.seat, helloBy: 0, recent: previous?.recent ?? [] } satisfies Attachment,
        );
        // The same seat on another socket is the old connection: drop it, so
        // one seat is one open socket.
        for (const ws of this.ctx.getWebSockets()) {
          if (ws === socket) continue;
          const ours = attachmentOf(ws);
          if (ours && ours.seat === effect.seat) closeSocket(ws, 4000, "replaced");
        }
        return;
      }

      case "send":
        for (const ws of this.ctx.getWebSockets()) {
          const ours = attachmentOf(ws);
          if (ours && ours.seat === effect.seat) this.trySend(ws, effect.msg);
        }
        return;

      case "broadcast":
        for (const ws of this.ctx.getWebSockets()) {
          const ours = attachmentOf(ws);
          if (ours && ours.seat !== 0 && ours.seat !== effect.except) this.trySend(ws, effect.msg);
        }
        return;

      case "close": {
        const sockets: WebSocket[] = [];
        if (effect.seat === null) {
          if (socket !== null) sockets.push(socket);
        } else {
          for (const ws of this.ctx.getWebSockets()) {
            const ours = attachmentOf(ws);
            if (ours && ours.seat === effect.seat) sockets.push(ws);
          }
        }
        for (const ws of sockets) closeSocket(ws, effect.code, effect.reason);
        return;
      }

      case "closeAll":
        for (const ws of this.ctx.getWebSockets()) closeSocket(ws, effect.code, effect.reason);
        return;

      case "record": {
        const row = effect.row;
        try {
          await this.env.DB.prepare(
            "INSERT INTO async_results (code, seat, key_hash, name, solved, guesses, elapsed_ms, rows, finished_at) " +
              "VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9) ON CONFLICT (code, seat) DO NOTHING",
          ).bind(
            row.code, row.seat, row.keyHash, row.name, row.solved ? 1 : 0,
            row.guesses, row.elapsedMs, JSON.stringify(row.rows), row.finishedAt,
          ).run();
        } catch {
          // A failure here is not a failed round: the player still gets their
          // result over the socket.
        }
        return;
      }

      case "delete":
        await this.ctx.storage.deleteAlarm();
        await this.ctx.storage.deleteAll();
        return;
    }
  }

  private trySend(ws: WebSocket, msg: ServerMessage): void {
    try {
      ws.send(JSON.stringify(msg));
    } catch {
      // A socket that is closing throws on send; its seat finds out through
      // the close event instead.
    }
  }

  // schedule arms the next alarm: the smallest of the room's own deadlines
  // and any unhellosed socket's hello deadline. An idle room holds no alarm.
  private async schedule(): Promise<void> {
    let at = nextDeadline(this.state);
    for (const ws of this.ctx.getWebSockets()) {
      const attachment = attachmentOf(ws);
      if (attachment && attachment.seat === 0 && attachment.helloBy > 0) {
        at = at === null ? attachment.helloBy : Math.min(at, attachment.helloBy);
      }
    }
    if (at !== null && at > 0) {
      await this.ctx.storage.setAlarm(at);
    } else {
      await this.ctx.storage.deleteAlarm();
    }
  }
}

interface InitBody {
  code: string;
  mode: "live" | "async";
  length: number;
  autoStart: boolean;
}

// attachmentOf reads a socket's memory without caring whether the socket is
// already halfway closed; a read that throws reads as "nothing attached".
function attachmentOf(ws: WebSocket): Attachment | null {
  try {
    return ws.deserializeAttachment() as Attachment | null;
  } catch {
    return null;
  }
}

function json(status: number, body: unknown): Response {
  const headers: Record<string, string> = {
    "Content-Type": "application/json; charset=utf-8",
    "Cache-Control": "no-store",
    "X-Content-Type-Options": "nosniff",
  };
  return new Response(JSON.stringify(body), { status, headers });
}

function closeSocket(ws: WebSocket, code: number, reason: string): void {
  try {
    ws.close(code, reason);
  } catch {
    // Closing a socket that is already gone is fine.
  }
}
