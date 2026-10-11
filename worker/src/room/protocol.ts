// The room protocol, Appendix A. Pure.
//
// ClientMessage is what a seat sends, ServerMessage what the room sends back.
// The wire is one JSON object per text frame; a parse that accepts anything
// unlisted here is the room's first line of defence, since every field is
// checked here before any rule uses it.

import { KEY_PATTERN } from "../codes.ts";

export type Mode = "live" | "async";
export type Phase = "lobby" | "countdown" | "playing" | "results" | "open";

export interface Hello { t: "hello"; v: number; client: string; name: string; key: string }
export type ClientMessage =
  | Hello
  | { t: "ready"; ready: boolean }
  | { t: "start" }
  | { t: "guess"; n: number; word: string }
  | { t: "rematch" }
  | { t: "leave" };

export interface PlayerView {
  seat: number; name: string; connected: boolean; ready: boolean; inRound: boolean;
  guesses: number; done: boolean; solved: boolean; rows: number[][];
}
export interface YouView {
  seat: number; inRound: boolean; words: string[]; marks: number[][];
  done: boolean; solved: boolean; elapsedMs: number;
}
export interface RankEntry {
  seat: number; name: string; solved: boolean; guesses: number; elapsedMs: number; rows: number[][];
}
export interface Results { round: number; word: string; ranking: RankEntry[] }
export interface RoomView {
  code: string; mode: Mode; length: number; autoStart: boolean; host: number;
  phase: Phase; round: number; goAt: number; endsAt: number; expiresAt: number;
  players: PlayerView[]; you: YouView; results: Results | null;
}

export type ServerMessage =
  | { t: "welcome"; v: 1; now: number; seat: number; key: string; room: RoomView }
  | { t: "refuse"; reason: "no_room" | "full" | "started" | "bad_key" | "left" }
  | { t: "gone"; message: string }
  | { t: "error"; reason: "unknown_type" | "bad_message" | "not_allowed" }
  | { t: "joined"; player: PlayerView }
  | { t: "left"; seat: number }
  | { t: "presence"; seat: number; connected: false }
  | { t: "ready"; seat: number; ready: boolean }
  | { t: "host"; seat: number }
  | { t: "countdown"; now: number; goAt: number; round: number }
  | { t: "go"; now: number; endsAt: number; round: number }
  | { t: "marks"; n: number; marks: number[] }
  | { t: "reject"; n: number; reason: "not_a_word" | "wrong_length" | "not_playing" | "out_of_order" }
  | { t: "progress"; seat: number; n: number; marks: number[] }
  | { t: "done"; seat: number; solved: boolean; guesses: number; elapsedMs: number }
  | { t: "results"; round: number; word: string; ranking: RankEntry[] }
  | { t: "lobby"; round: number };

export type Parsed =
  | { ok: true; msg: ClientMessage }
  | { ok: false; reason: "unknown_type" | "bad_message" | "not_json" };

const TYPES = new Set(["hello", "ready", "start", "guess", "rematch", "leave"]);

// parseClientMessage checks the shape of one text frame. The returned message
// carries only the fields its type names: nothing a client added beyond them
// reaches the room. (Hello's v is checked for 1 later; it is still a shape.)
export function parseClientMessage(text: string): Parsed {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { ok: false, reason: "not_json" };
  }
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { ok: false, reason: "not_json" };
  }
  const raw = value as Record<string, unknown>;
  const t = raw.t;
  if (typeof t !== "string" || !TYPES.has(t)) {
    return { ok: false, reason: "unknown_type" };
  }

  switch (t) {
    case "hello": {
      const v = raw.v;
      const client = raw.client;
      const name = raw.name;
      const key = raw.key;
      if (
        typeof v !== "number" ||
        typeof client !== "string" || client.length < 1 || client.length > 100 ||
        typeof name !== "string" || name.length > 64 ||
        typeof key !== "string" || (key !== "" && !KEY_PATTERN.test(key))
      ) {
        return { ok: false, reason: "bad_message" };
      }
      const msg: Hello = { t, v, client, name, key };
      return { ok: true, msg };
    }
    case "ready": {
      const ready = raw.ready;
      if (typeof ready !== "boolean") return { ok: false, reason: "bad_message" };
      return { ok: true, msg: { t, ready } };
    }
    case "guess": {
      const n = raw.n;
      const word = raw.word;
      if (
        typeof n !== "number" || !Number.isInteger(n) || n < 1 || n > 7 ||
        typeof word !== "string" || word.length < 1 || word.length > 16
      ) {
        return { ok: false, reason: "bad_message" };
      }
      return { ok: true, msg: { t, n, word } };
    }
    default: {
      // start, rematch, leave: no fields checked.
      return { ok: true, msg: { t } as ClientMessage };
    }
  }
}
