// The whole room state machine, Appendix A. Pure.
//
// handle(state, event, ctx) → { state, effects } is where every rule lives.
// It never touches a socket, storage, D1, the clock or randomness directly:
// the time and the random answer arrive through ctx, and everything that must
// happen is described as a list of effects for room.ts to carry out, in
// order. Every rule is tested here in plain Node.

import { cleanName } from "../names.ts";
import { isGuess } from "../words.ts";
import { score } from "../score.ts";
import { rank } from "./rank.ts";
import type {
  ClientMessage, Hello, Mode, Phase, PlayerView, RankEntry, Results, RoomView, ServerMessage,
} from "./protocol.ts";

export const LIVE_MAX = 8;
export const LIVE_MIN = 2;
export const ASYNC_MAX = 50;
export const ROUND_MS = 600_000;
export const COUNTDOWN_MS = 3_000;
export const GRACE_MS = 120_000;
export const KEEP_AFTER_ROUND_MS = 600_000;
export const ASYNC_LIFETIME_MS = 2_592_000_000;

const LETTERS = /^[a-z]+$/;

export interface Seat {
  seat: number;
  name: string;
  keyHash: string;
  joinedAt: number;
  removed: boolean;
  ready: boolean;
  inRound: boolean;
  words: string[];
  marks: number[][];
  done: boolean;
  solved: boolean;
  elapsedMs: number;
  startedAt: number; // async only; 0 = not started
  endsAt: number; // async only; 0 = not started
}

export interface RoomState {
  v: 1;
  code: string;
  mode: Mode;
  length: number;
  autoStart: boolean;
  createdAt: number;
  expiresAt: number; // async; 0 for live
  host: number; // live; 0 for async
  phase: Phase;
  round: number;
  answer: string; // "" when no word is chosen (live lobby)
  goAt: number;
  endsAt: number; // live round end
  lastRoundEndedAt: number;
  deleteAt: number; // live; 0 = not scheduled
  nextSeat: number;
  seats: Seat[];
  results: Results | null;
}

export interface Ctx {
  now: number;
  // Seats with at least one open, attached socket when the event happened. For
  // a disconnect event, the closing socket is already left out.
  connected: ReadonlySet<number>;
  // A random answer for a length. Tests pass a fixed word.
  pickAnswer: (length: number) => string;
}

export interface ResultRow {
  code: string;
  seat: number;
  keyHash: string;
  name: string;
  solved: boolean;
  guesses: number;
  elapsedMs: number;
  rows: number[][];
  finishedAt: number;
}

export type Event =
  | { type: "hello"; msg: Hello; keyHash: string | null; fresh: { key: string; keyHash: string } }
  | { type: "message"; seat: number; msg: Exclude<ClientMessage, Hello> }
  | { type: "disconnect"; seat: number }
  | { type: "alarm" };

type HelloEvent = Extract<Event, { type: "hello" }>;

export type Effect =
  | { kind: "reply"; msg: ServerMessage } // to the socket that sent this event
  | { kind: "attach"; seat: number } // bind the hello socket to this seat
  | { kind: "send"; seat: number; msg: ServerMessage } // to every socket of this seat
  | { kind: "broadcast"; msg: ServerMessage; except?: number } // to every attached socket, optionally not this seat
  | { kind: "close"; seat: number | null; code: number; reason: string } // null = the event's socket
  | { kind: "closeAll"; code: number; reason: string }
  | { kind: "record"; row: ResultRow }
  | { kind: "delete" };

export function createRoom(
  opts: { code: string; mode: Mode; length: number; autoStart: boolean },
  ctx: Ctx,
  creator: { keyHash: string },
): RoomState {
  return {
    v: 1,
    code: opts.code,
    mode: opts.mode,
    length: opts.length,
    autoStart: opts.mode === "live" ? opts.autoStart : false,
    createdAt: ctx.now,
    expiresAt: opts.mode === "async" ? ctx.now + ASYNC_LIFETIME_MS : 0,
    host: opts.mode === "live" ? 1 : 0,
    phase: opts.mode === "live" ? "lobby" : "open",
    round: 1,
    answer: opts.mode === "async" ? ctx.pickAnswer(opts.length) : "",
    goAt: 0,
    endsAt: 0,
    lastRoundEndedAt: 0,
    deleteAt: 0,
    nextSeat: 2,
    seats: [{
      seat: 1,
      name: "",
      keyHash: creator.keyHash,
      joinedAt: ctx.now,
      removed: false,
      ready: false,
      inRound: false,
      words: [],
      marks: [],
      done: false,
      solved: false,
      elapsedMs: 0,
      startedAt: 0,
      endsAt: 0,
    }],
    results: null,
  };
}

// handle implements Appendix A sections A.3 (allowed-when column), A.6, A.8
// and A.9. It never mutates its input: a change happens on a clone.
export function handle(
  state: RoomState | null,
  event: Event,
  ctx: Ctx,
): { state: RoomState | null; effects: Effect[] } {
  if (state === null) {
    // No room: only a hello gets an answer, and it is always no_room. Any
    // other event is from a socket the runtime dropped anyway.
    if (event.type === "hello") {
      const effects: Effect[] = [];
      refuse(effects, "no_room");
      return { state: null, effects };
    }
    return { state: null, effects: [] };
  }

  const s: RoomState = structuredClone(state);
  const effects: Effect[] = [];

  switch (event.type) {
    case "hello":
      return { state: onHello(s, event, ctx, effects), effects };
    case "message":
      onMessage(s, event.seat, event.msg, ctx, effects);
      return { state: s, effects };
    case "disconnect":
      onDisconnect(s, event.seat, ctx, effects);
      return { state: s, effects };
    case "alarm":
      return onAlarm(s, ctx, effects);
  }
}

// refuse answers a hello the room will not take, and drops the socket. The
// room itself is unchanged.
function refuse(effects: Effect[], reason: HelloRefuse): void {
  effects.push({ kind: "reply", msg: { t: "refuse", reason } });
  effects.push({ kind: "close", seat: null, code: 4002, reason: "refused" });
}

type HelloRefuse = "no_room" | "full" | "started" | "bad_key" | "left";

function findSeat(s: RoomState, seat: number): Seat | undefined {
  return s.seats.find((p) => p.seat === seat);
}

// nonRemoved is the room's living membership: removed seats are gone for
// every capacity, host and listing decision.
function nonRemoved(s: RoomState): Seat[] {
  return s.seats.filter((p) => !p.removed);
}

function playerView(s: RoomState, seat: number, connected: ReadonlySet<number>): PlayerView {
  const p = findSeat(s, seat)!;
  if (s.mode === "async") {
    // An async room says nothing about a round but whether it was finished.
    return { seat, name: p.name, connected: false, ready: false, inRound: false,
      guesses: 0, done: p.done, solved: false, rows: [] };
  }
  return {
    seat,
    name: p.name,
    connected: connected.has(seat),
    ready: p.ready,
    inRound: p.inRound,
    guesses: p.marks.length,
    done: p.done,
    solved: p.solved,
    rows: p.marks,
  };
}

// rankEntry is one results line: marks, never letters.
function rankEntry(p: Seat): RankEntry {
  return {
    seat: p.seat,
    name: p.name,
    solved: p.solved,
    guesses: p.words.length,
    elapsedMs: p.elapsedMs,
    rows: p.marks,
  };
}

// roomView is the RoomView of Appendix A.5, for the asking seat. `connected`
// must be the seats holding an open socket at this moment; a caller that is
// itself welcoming a seat passes the set with that seat added.
export function roomView(state: RoomState, seat: number, connected: ReadonlySet<number>): RoomView {
  const s = state;
  const you = findSeat(s, seat)!;
  return {
    code: s.code,
    mode: s.mode,
    length: s.length,
    autoStart: s.autoStart,
    host: s.host,
    phase: s.phase,
    round: s.round,
    goAt: s.goAt,
    endsAt: s.mode === "live" ? s.endsAt : you.endsAt,
    expiresAt: s.expiresAt,
    players: nonRemoved(s).sort((a, b) => a.seat - b.seat).map((p) => playerView(s, p.seat, connected)),
    you: {
      seat,
      inRound: s.mode === "live" ? you.inRound : you.startedAt > 0,
      words: [...you.words],
      marks: you.marks.map((row) => [...row]),
      done: you.done,
      solved: you.solved,
      elapsedMs: you.elapsedMs,
    },
    // A live room's results are the room's; an async room's are each
    // player's own, once they have finished: the ranking so far, the same
    // one their finish was answered with.
    results: s.mode === "live"
      ? s.results
      : you.done ? { round: s.round, word: s.answer, ranking: asyncRanking(s) } : null,
  };
}

// asyncRanking is an async room's ranking so far: every seat that finished,
// including one that has left since — it played, and the results endpoint,
// which reads D1, counts it the same way.
function asyncRanking(s: RoomState): RankEntry[] {
  return rank(s.seats.filter((p) => p.done).map(rankEntry));
}

// startCondition, A.8: at least LIVE_MIN connected seats, and every one of
// them ready. The name covers the live lobby; an async room never checks it.
function startCondition(s: RoomState, connected: ReadonlySet<number>): boolean {
  const inLobby = nonRemoved(s).filter((p) => connected.has(p.seat));
  return inLobby.length >= LIVE_MIN && inLobby.every((p) => p.ready);
}

function startCountdown(s: RoomState, effects: Effect[], ctx: Ctx): void {
  s.answer = ctx.pickAnswer(s.length);
  s.phase = "countdown";
  s.goAt = ctx.now + COUNTDOWN_MS;
  s.endsAt = 0;
  s.results = null;
  for (const p of s.seats) {
    p.inRound = ctx.connected.has(p.seat) && !p.removed;
    p.words = [];
    p.marks = [];
    p.done = false;
    p.solved = false;
    p.elapsedMs = 0;
  }
  effects.push({ kind: "broadcast", msg: { t: "countdown", now: ctx.now, goAt: s.goAt, round: s.round } });
}

// endRound, A.8: the round is over and the word is spoken. A room nobody is
// connected to then keeps itself only for the settled minimum grace.
function endRound(s: RoomState, effects: Effect[], ctx: Ctx): void {
  s.phase = "results";
  s.lastRoundEndedAt = ctx.now;
  const ranking = rank(s.seats.filter((p) => p.inRound).map(rankEntry));
  s.results = { round: s.round, word: s.answer, ranking };
  effects.push({ kind: "broadcast", msg: { t: "results", round: s.round, word: s.answer, ranking } });
  if (ctx.connected.size === 0) {
    s.deleteAt = Math.max(ctx.now + GRACE_MS, s.lastRoundEndedAt + KEEP_AFTER_ROUND_MS);
  }
}

// rematch, A.8: back to the lobby with a fresh round number. The start
// condition is deliberately not re-checked: every ready flag is false.
function rematch(s: RoomState, effects: Effect[]): void {
  s.round += 1;
  s.phase = "lobby";
  s.answer = "";
  s.goAt = 0;
  s.endsAt = 0;
  s.results = null;
  for (const p of s.seats) {
    p.ready = false;
    p.inRound = false;
    p.words = [];
    p.marks = [];
    p.done = false;
    p.solved = false;
    p.elapsedMs = 0;
  }
  effects.push({ kind: "broadcast", msg: { t: "lobby", round: s.round } });
}

function onHello(s: RoomState, event: HelloEvent, ctx: Ctx, effects: Effect[]): RoomState {
  const msg = event.msg;

  let seat: Seat | undefined;
  let seatKey = "";
  if (msg.key !== "") {
    // A key answers for a seat that already exists: its hash was checked by
    // the socket layer before this event.
    seat = s.seats.find((p) => p.keyHash === event.keyHash);
    if (!seat) {
      refuse(effects, "bad_key");
      return s;
    }
    if (seat.removed) {
      refuse(effects, "left");
      return s;
    }
    seat.name = cleanName(msg.name);
    seatKey = msg.key;
  } else {
    // A new seat: capacity and phase decide whether the room takes it.
    if (s.mode === "live") {
      if (s.phase !== "lobby") {
        refuse(effects, "started");
        return s;
      }
      if (nonRemoved(s).length >= LIVE_MAX) {
        refuse(effects, "full");
        return s;
      }
    } else {
      if (ctx.now >= s.expiresAt || nonRemoved(s).length >= ASYNC_MAX) {
        refuse(effects, "full");
        return s;
      }
    }
    seat = {
      seat: s.nextSeat++,
      name: cleanName(msg.name),
      keyHash: event.fresh.keyHash,
      joinedAt: ctx.now,
      removed: false,
      ready: false,
      inRound: false,
      words: [],
      marks: [],
      done: false,
      solved: false,
      elapsedMs: 0,
      startedAt: 0,
      endsAt: 0,
    };
    s.seats.push(seat);
    seatKey = event.fresh.key;
  }

  // A hello means somebody is here: nothing is scheduled for deletion.
  s.deleteAt = 0;

  // The joining seat is connected from this moment — the same set answers for
  // the welcome's view and for every check the event makes.
  const connectedPlusS = new Set(ctx.connected);
  connectedPlusS.add(seat.seat);

  effects.push({ kind: "attach", seat: seat.seat });
  effects.push({
    kind: "reply",
    msg: {
      t: "welcome",
      v: 1,
      now: ctx.now,
      seat: seat.seat,
      key: seatKey,
      room: roomView(s, seat.seat, connectedPlusS),
    },
  });
  if (s.mode === "live") {
    // An async room never announces players; a live room does, with the
    // newcomer's own socket answered above before this could reach it.
    effects.push({
      kind: "broadcast",
      msg: { t: "joined", player: playerView(s, seat.seat, connectedPlusS) },
      except: seat.seat,
    });
    if (s.phase === "lobby" && s.autoStart && startCondition(s, connectedPlusS)) {
      // The countdown takes its players from the same set the condition was
      // checked on: ctx.connected does not hold the seat saying hello yet, and
      // a ready player whose return completes the room must be in the round
      // it starts.
      startCountdown(s, effects, { ...ctx, connected: connectedPlusS });
    }
  }
  return s;
}

function onMessage(
  s: RoomState,
  seatNumber: number,
  msg: Exclude<ClientMessage, Hello>,
  ctx: Ctx,
  effects: Effect[],
): void {
  const seat = findSeat(s, seatNumber);
  if (!seat) {
    effects.push({ kind: "reply", msg: { t: "error", reason: "not_allowed" } });
    return;
  }
  const notAllowed = (target: number) => effects.push({ kind: "send", seat: target, msg: { t: "error", reason: "not_allowed" } });

  switch (msg.t) {
    case "ready": {
      // Allowed when: live, phase lobby.
      if (s.mode !== "live" || s.phase !== "lobby") {
        notAllowed(seat.seat);
        return;
      }
      seat.ready = msg.ready;
      effects.push({ kind: "broadcast", msg: { t: "ready", seat: seat.seat, ready: seat.ready } });
      if (s.autoStart && startCondition(s, ctx.connected)) {
        startCountdown(s, effects, ctx);
      }
      return;
    }

    case "start": {
      if (s.mode === "live") {
        // Allowed when: lobby, host, non-auto rooms, start condition.
        if (s.phase !== "lobby" || s.host !== seat.seat || s.autoStart || !startCondition(s, ctx.connected)) {
          notAllowed(seat.seat);
          return;
        }
        startCountdown(s, effects, ctx);
        return;
      }
      // Async: a seat starts its own private round.
      if (seat.startedAt !== 0) {
        notAllowed(seat.seat);
        return;
      }
      seat.startedAt = ctx.now;
      seat.endsAt = ctx.now + ROUND_MS;
      seat.inRound = true;
      effects.push({ kind: "send", seat: seat.seat, msg: { t: "go", now: ctx.now, endsAt: seat.endsAt, round: 1 } });
      return;
    }

    case "guess":
      onGuess(s, seat, msg.n, msg.word, ctx, effects);
      return;

    case "rematch": {
      // Allowed when: live, phase results.
      if (s.mode !== "live" || s.phase !== "results") {
        notAllowed(seat.seat);
        return;
      }
      rematch(s, effects);
      return;
    }

    case "leave":
      leave(s, seat, ctx, effects);
      return;
  }
}

function onGuess(
  s: RoomState,
  seat: Seat,
  n: number,
  word: string,
  ctx: Ctx,
  effects: Effect[],
): void {
  if (s.mode === "live") {
    // Live: a round is running only in phase playing.
    if (s.phase !== "playing" || !seat.inRound || seat.done) {
      reject(effects, seat.seat, n, "not_playing");
      return;
    }
  } else {
    if (seat.startedAt === 0 || seat.done) {
      reject(effects, seat.seat, n, "not_playing");
      return;
    }
    if (ctx.now >= seat.endsAt) {
      // Out of time first — A.6 rule 7 for the seat — then the refusal.
      finishAsync(s, seat, effects, ctx, false);
      reject(effects, seat.seat, n, "not_playing");
      return;
    }
  }

  // An already-accepted guess is answered again unchanged (A.6 rule 2); this
  // is what makes a resend after a reconnect safe.
  if (n <= seat.words.length) {
    effects.push({ kind: "send", seat: seat.seat, msg: { t: "marks", n, marks: seat.marks[n - 1] } });
    return;
  }
  if (n !== seat.words.length + 1) {
    reject(effects, seat.seat, n, "out_of_order");
    return;
  }
  if (!LETTERS.test(word) || word.length !== s.length) {
    reject(effects, seat.seat, n, "wrong_length");
    return;
  }
  if (!isGuess(s.length, word) && word !== s.answer) {
    reject(effects, seat.seat, n, "not_a_word");
    return;
  }

  const marks = score(word, s.answer);
  seat.marks.push([...marks]);
  seat.words.push(word);
  effects.push({ kind: "send", seat: seat.seat, msg: { t: "marks", n, marks: [...marks] } });
  if (s.mode === "live") {
    // Other seats see colours, never the word; their own guesses stay theirs.
    effects.push({
      kind: "broadcast",
      msg: { t: "progress", seat: seat.seat, n, marks: [...marks] },
      except: seat.seat,
    });
  }

  // A.6 rule 7: the word hit, or the slot marked "out of guesses" is spent.
  if (word === s.answer || seat.words.length >= s.length + 1) {
    const solved = word === s.answer;
    const startMs = s.mode === "live" ? s.goAt : seat.startedAt;
    seat.done = true;
    seat.solved = solved;
    seat.elapsedMs = clamp(ctx.now - startMs);
    if (s.mode === "live") {
      effects.push({
        kind: "broadcast",
        msg: { t: "done", seat: seat.seat, solved, guesses: seat.words.length, elapsedMs: seat.elapsedMs },
      });
      if (s.seats.filter((p) => p.inRound).every((p) => p.done)) {
        endRound(s, effects, ctx);
      }
    } else {
      finishAsync(s, seat, effects, ctx, solved);
    }
  }
}

function reject(
  effects: Effect[],
  seat: number,
  n: number,
  reason: "not_a_word" | "wrong_length" | "not_playing" | "out_of_order",
): void {
  effects.push({ kind: "send", seat, msg: { t: "reject", n, reason } });
}

function finishAsync(s: RoomState, seat: Seat, effects: Effect[], ctx: Ctx, solved: boolean): void {
  seat.done = true;
  seat.solved = solved;
  seat.elapsedMs = clamp(ctx.now - seat.startedAt);
  effects.push({ kind: "record", row: record(s, seat, ctx) });
  if (ctx.connected.has(seat.seat)) {
    effects.push({
      kind: "send",
      seat: seat.seat,
      msg: { t: "done", seat: seat.seat, solved, guesses: seat.words.length, elapsedMs: seat.elapsedMs },
    });
    effects.push({
      kind: "send",
      seat: seat.seat,
      msg: {
        t: "results",
        round: s.round,
        word: s.answer,
        ranking: asyncRanking(s),
      },
    });
  }
}

const clamp = (ms: number): number => Math.max(0, Math.min(ms, ROUND_MS));

function record(s: RoomState, seat: Seat, ctx: Ctx): ResultRow {
  return {
    code: s.code,
    seat: seat.seat,
    keyHash: seat.keyHash,
    name: seat.name,
    solved: seat.solved,
    guesses: seat.words.length,
    elapsedMs: seat.elapsedMs,
    rows: seat.marks,
    finishedAt: ctx.now,
  };
}

// leave, A.8 for a live room and A.9 for an async one: finished mid-round if
// one was running, then removed for good with the sockets closed and, live,
// the host seat passed on.
function leave(s: RoomState, seat: Seat, ctx: Ctx, effects: Effect[]): void {
  if (s.mode === "live") {
    if (seat.inRound && !seat.done && (s.phase === "countdown" || s.phase === "playing")) {
      seat.done = true;
      seat.solved = false;
      seat.elapsedMs = clamp(ctx.now - s.goAt);
      effects.push({
        kind: "broadcast",
        msg: { t: "done", seat: seat.seat, solved: false, guesses: seat.words.length, elapsedMs: seat.elapsedMs },
      });
    }
    seat.removed = true;
    // The leave goes to every socket, the leaving one included: it is closed
    // right below, so what reaches it last does not matter.
    effects.push({ kind: "broadcast", msg: { t: "left", seat: seat.seat } });
    effects.push({ kind: "close", seat: seat.seat, code: 1000, reason: "left" });
    if (s.host === seat.seat) {
      const remaining = nonRemoved(s).map((p) => p.seat);
      if (remaining.length > 0) {
        s.host = Math.min(...remaining);
        effects.push({ kind: "broadcast", msg: { t: "host", seat: s.host } });
      }
    }
    if (s.phase === "playing" && s.seats.filter((p) => p.inRound).every((p) => p.done)) {
      endRound(s, effects, ctx);
    } else if (s.phase === "lobby" && s.autoStart && startCondition(s, ctx.connected)) {
      startCountdown(s, effects, ctx);
    }
    return;
  }
  // Async, A.9: a started seat is finished (never solved, time capped) and
  // recorded; an unstarted one is simply removed. No message is broadcast:
  // an async room's membership changes are private.
  if (seat.startedAt !== 0 && !seat.done) {
    seat.done = true;
    seat.solved = false;
    seat.elapsedMs = clamp(ctx.now - seat.startedAt);
    effects.push({ kind: "record", row: record(s, seat, ctx) });
  }
  seat.removed = true;
  effects.push({ kind: "close", seat: seat.seat, code: 1000, reason: "left" });
}

function onDisconnect(s: RoomState, seatNumber: number, ctx: Ctx, effects: Effect[]): void {
  if (s.mode !== "live") {
    // An async room's seats come and go; nothing announces it, and its rounds
    // are timed by alarms rather than by who is connected.
    return;
  }
  if (!ctx.connected.has(seatNumber)) {
    effects.push({ kind: "broadcast", msg: { t: "presence", seat: seatNumber, connected: false } });
  }
  if (s.phase === "lobby" && s.autoStart && startCondition(s, ctx.connected)) {
    startCountdown(s, effects, ctx);
  }
  if (ctx.connected.size === 0 && (s.phase === "lobby" || s.phase === "results")) {
    s.deleteAt = Math.max(ctx.now + GRACE_MS, s.lastRoundEndedAt + KEEP_AFTER_ROUND_MS);
  }
}

function onAlarm(s: RoomState, ctx: Ctx, effects: Effect[]): { state: RoomState | null; effects: Effect[] } {
  if (s.mode === "live") {
    if (s.phase === "countdown" && ctx.now >= s.goAt) {
      s.phase = "playing";
      s.endsAt = s.goAt + ROUND_MS;
      effects.push({ kind: "broadcast", msg: { t: "go", now: ctx.now, endsAt: s.endsAt, round: s.round } });
    }
    if (s.phase === "playing" && ctx.now >= s.endsAt) {
      for (const p of s.seats) {
        if (p.inRound && !p.done) {
          p.done = true;
          p.solved = false;
          p.elapsedMs = ROUND_MS;
          effects.push({
            kind: "broadcast",
            msg: { t: "done", seat: p.seat, solved: false, guesses: p.words.length, elapsedMs: p.elapsedMs },
          });
        }
      }
      endRound(s, effects, ctx);
    }
    if (s.deleteAt > 0 && ctx.now >= s.deleteAt && ctx.connected.size === 0) {
      effects.push({ kind: "delete" });
      // The room is gone: its state is not kept.
      return { state: null, effects };
    }
    return { state: s, effects };
  }

  for (const p of s.seats) {
    if (p.startedAt > 0 && !p.done && ctx.now >= p.endsAt) {
      p.done = true;
      p.solved = false;
      p.elapsedMs = ROUND_MS;
      effects.push({ kind: "record", row: record(s, p, ctx) });
      if (ctx.connected.has(p.seat)) {
        effects.push({
          kind: "send",
          seat: p.seat,
          msg: { t: "done", seat: p.seat, solved: false, guesses: p.words.length, elapsedMs: p.elapsedMs },
        });
        effects.push({
          kind: "send",
          seat: p.seat,
          msg: {
            t: "results",
            round: s.round,
            word: s.answer,
            ranking: asyncRanking(s),
          },
        });
      }
    }
  }
  if (ctx.now >= s.expiresAt) {
    effects.push({ kind: "closeAll", code: 4004, reason: "room closed" });
    effects.push({ kind: "delete" });
    return { state: null, effects };
  }
  return { state: s, effects };
}

// nextDeadline is the room's next alarm: nothing, or the smallest time any
// rule needs waking for. The socket layer adds its hello deadlines on top of
// whatever this returns.
export function nextDeadline(state: RoomState | null): number | null {
  if (state === null) return null;
  const s = state;
  const times: number[] = [];
  if (s.mode === "live") {
    if (s.phase === "countdown") times.push(s.goAt);
    if (s.phase === "playing") times.push(s.endsAt);
    if (s.deleteAt > 0) times.push(s.deleteAt);
  } else {
    times.push(s.expiresAt);
    for (const p of s.seats) {
      if (p.startedAt > 0 && !p.done) times.push(p.endsAt);
    }
  }
  const running = times.filter((v) => v > 0);
  return running.length === 0 ? null : Math.min(...running);
}
