import { test } from "node:test";
import assert from "node:assert/strict";
import type { Ctx, Effect, RoomState } from "../src/room/state.ts";
import {
  ASYNC_LIFETIME_MS, COUNTDOWN_MS, GRACE_MS, KEEP_AFTER_ROUND_MS,
  ROUND_MS, createRoom, handle, nextDeadline,
} from "../src/room/state.ts";
import type { Hello } from "../src/room/protocol.ts";
import { score } from "../src/score.ts";

const WORD = "crane";

// Hashes are plain strings in tests; handle never hashes.
const KEY1 = { key: "1".repeat(32), keyHash: "h1" };
const KEY2 = { key: "2".repeat(32), keyHash: "h2" };
const KEY3 = { key: "3".repeat(32), keyHash: "h3" };

// T is a fixed start time: an ordinary epoch moment, so all of the grace
// arithmetic runs against numbers far above zero.
const T = 1_771_000_000_000;

function ctx(now: number, connected: number[] = []): Ctx {
  return { now, connected: new Set(connected), pickAnswer: () => WORD };
}

function hello(name: string, key = ""): Hello {
  return { t: "hello", v: 1, client: "surmise/0.10.0 (linux/amd64)", name, key };
}

// sent returns every message of type t in reply/send/broadcast effects.
function sent(effects: Effect[], t: string): { t: string }[] {
  const out: { t: string }[] = [];
  for (const e of effects) {
    if ((e.kind === "reply" || e.kind === "send" || e.kind === "broadcast") && e.msg.t === t) {
      out.push(e.msg);
    }
  }
  return out;
}

// ofKind returns the whole effects of one kind.
function ofKind(effects: Effect[], kind: string): Effect[] {
  return effects.filter((e) => e.kind === kind);
}

// start builds a live room and brings seat 2 in; nobody is ready yet.
function start(autoStart: boolean): RoomState {
  const fresh = createRoom({ code: "K7M2QX", mode: "live", length: 5, autoStart }, ctx(T), { keyHash: "h1" });
  const joined = handle(fresh, { type: "hello", msg: hello("sam"), keyHash: null, fresh: KEY2 }, ctx(T, []));
  return joined.state as RoomState;
}

// readied marks both seats ready; in an autoStart room the second ready is
// what starts the countdown, and this function asserts that.
function readied(state: RoomState, autoStart: boolean): RoomState {
  const first = handle(state, { type: "message", seat: 1, msg: { t: "ready", ready: true } }, ctx(T, [1, 2]));
  if (autoStart) assert.equal(sent(first.effects, "countdown").length, 0, "one ready seat starts nothing");
  const second = handle(first.state as RoomState, { type: "message", seat: 2, msg: { t: "ready", ready: true } },
    ctx(T, [1, 2]));
  if (autoStart) assert.equal(sent(second.effects, "countdown").length, 1, "the second ready must start it");
  else assert.equal(sent(second.effects, "countdown").length, 0, "a host-started room waits for the host");
  return second.state as RoomState;
}

const GO_AT = T + COUNTDOWN_MS;
const ENDS_AT = GO_AT + ROUND_MS;

// playing takes a room through countdown and go: phase playing, no guesses.
function playing(autoStart: boolean): RoomState {
  const counted = readied(start(autoStart), autoStart);
  const gone = handle(counted, { type: "alarm" }, ctx(GO_AT, [1, 2]));
  assert.equal(sent(gone.effects, "go").length, 1, "the alarm started the round");
  return gone.state as RoomState;
}

// startAsync builds an async room with seat 1 present.
function startAsync(): RoomState {
  return createRoom({ code: "K7M2QX", mode: "async", length: 5, autoStart: false }, ctx(T), { keyHash: "h1" });
}

function helloIn(state: RoomState | null, name: string, key: string, keyHash: string | null,
  fresh: { key: string; keyHash: string }, now: number, connected: number[]) {
  return handle(state, { type: "hello", msg: hello(name, key), keyHash, fresh }, ctx(now, connected));
}

function messageIn(state: RoomState, seat: number, msg: Record<string, unknown>, now: number,
  connected: number[]) {
  return handle(state, { type: "message", seat, msg } as unknown as Parameters<typeof handle>[1],
    ctx(now, connected));
}

// ---- live rooms ----

test("the creator rejoins with the host key", () => {
  const fresh = createRoom({ code: "K7M2QX", mode: "live", length: 5, autoStart: true }, ctx(T), { keyHash: "h1" });
  const r = helloIn(fresh, "nick", KEY1.key, "h1", KEY1, T + 10, []);
  assert.deepEqual(r.effects[0], { kind: "attach", seat: 1 });
  const welcome = sent(r.effects, "welcome")[0] as unknown as { seat: number; room: Record<string, unknown> };
  assert.equal(welcome.seat, 1);
  assert.equal(welcome.room.phase, "lobby");
  assert.equal(welcome.room.host, 1);
});

test("a new player gets a seat and the room hears of it", () => {
  const fresh = createRoom({ code: "K7M2QX", mode: "live", length: 5, autoStart: true }, ctx(T), { keyHash: "h1" });
  const host = helloIn(fresh, "nick", KEY1.key, "h1", KEY1, T + 10, []);
  const r = helloIn(host.state as RoomState, "sam", "", null, KEY2, T + 20, [1]);
  assert.deepEqual(r.effects[0], { kind: "attach", seat: 2 });
  const welcome = sent(r.effects, "welcome")[0] as unknown as { seat: number; key: string };
  assert.equal(welcome.seat, 2);
  assert.equal(welcome.key, KEY2.key, "a new seat is told its own key");
  const joined = sent(r.effects, "joined")[0] as unknown as { player: { seat: number } };
  assert.ok(joined, "the room hears of the join");
  assert.equal(joined.player.seat, 2);
  const joinedEffect = ofKind(r.effects, "broadcast").find(
    (e) => (e as { msg: { t: string } }).msg.t === "joined",
  ) as { except: number } | undefined;
  assert.equal(joinedEffect?.except, 2);
});

test("a wrong key is refused", () => {
  const state = start(true);
  const r = helloIn(state, "who?", KEY1.key, "nope", KEY3, T + 30, []);
  assert.deepEqual(sent(r.effects, "refuse")[0], { t: "refuse", reason: "bad_key" });
  assert.deepEqual(r.effects[1], { kind: "close", seat: null, code: 4002, reason: "refused" });
});

test("no room means no_room", () => {
  const r = helloIn(null, "anyone", "", null, KEY1, T, []);
  assert.equal(r.state, null);
  assert.deepEqual(sent(r.effects, "refuse")[0], { t: "refuse", reason: "no_room" });
  assert.deepEqual(r.effects[1], { kind: "close", seat: null, code: 4002, reason: "refused" });
});

test("the ninth player is refused", () => {
  let state = createRoom({ code: "K7M2QX", mode: "live", length: 5, autoStart: true }, ctx(T), { keyHash: "h1" });
  for (let seat = 2; seat <= 8; seat++) {
    const fresh = { key: String(seat).repeat(32), keyHash: "h" + seat };
    const r = helloIn(state, "p" + seat, "", null, fresh, T, []);
    state = r.state as RoomState;
  }
  const r = helloIn(state, "p9", "", null, { key: "9".repeat(32), keyHash: "h9" }, T, []);
  assert.deepEqual(sent(r.effects, "refuse")[0], { t: "refuse", reason: "full" });
  assert.equal((r.state as RoomState).seats.length, 8, "the refusal added no seat");
});

test("joining a running round is refused", () => {
  const round = playing(true);
  const r = helloIn(round, "late", "", null, KEY3, T + COUNTDOWN_MS + 5, [1, 2]);
  assert.deepEqual(sent(r.effects, "refuse")[0], { t: "refuse", reason: "started" });
});

test("one ready player does not start", () => {
  const fresh = createRoom({ code: "K7M2QX", mode: "live", length: 5, autoStart: true }, ctx(T), { keyHash: "h1" });
  const r = messageIn(fresh, 1, { t: "ready", ready: true }, T + 1, [1]);
  assert.equal(sent(r.effects, "countdown").length, 0);
});

test("two ready players start the countdown", () => {
  const state = start(true);
  const first = messageIn(state, 1, { t: "ready", ready: true }, T, [1, 2]);
  const second = messageIn(first.state as RoomState, 2, { t: "ready", ready: true }, T, [1, 2]);
  const countdowns = sent(second.effects, "countdown");
  assert.equal(countdowns.length, 1);
  const countdown = countdowns[0] as { goAt: number };
  assert.equal(countdown.goAt, T + COUNTDOWN_MS);
  const after = second.state as RoomState;
  assert.equal(after.answer, WORD);
  assert.equal(after.phase, "countdown");
  for (const seat of after.seats) {
    assert.equal(seat.inRound, true, "seat " + seat.seat + " is in round");
  }
});

test("host start waits for the host", () => {
  let state = start(false);
  const ready1 = messageIn(state, 1, { t: "ready", ready: true }, T, [1]);
  state = ready1.state as RoomState;
  const ready2 = messageIn(state, 2, { t: "ready", ready: true }, T, [1, 2]);
  state = ready2.state as RoomState;
  assert.equal(sent(ready2.effects, "countdown").length, 0);

  const byOther = messageIn(state, 2, { t: "start" }, T + 1, [1, 2]);
  assert.deepEqual(sent(byOther.effects, "error")[0], { t: "error", reason: "not_allowed" });

  const byHost = messageIn(state, 1, { t: "start" }, T + 2, [1, 2]);
  assert.equal(sent(byHost.effects, "countdown").length, 1);
});

test("the alarm starts the round", () => {
  const counted = readied(start(true), true);
  const r = handle(counted, { type: "alarm" }, ctx(GO_AT, [1, 2]));
  const gos = sent(r.effects, "go");
  assert.equal(gos.length, 1);
  const go = gos[0] as { endsAt: number };
  assert.equal(go.endsAt, GO_AT + ROUND_MS);
  assert.equal((r.state as RoomState).phase, "playing");
});

test("guesses are checked in order", () => {
  let state = playing(true);
  const at = T + COUNTDOWN_MS + 1000;
  const connected = [1, 2];

  // A guess out of order is refused before anything else.
  let r = messageIn(state, 1, { t: "guess", n: 2, word: "slate" }, at, connected);
  assert.deepEqual(sent(r.effects, "reject")[0], { t: "reject", n: 2, reason: "out_of_order" });
  state = r.state as RoomState;

  // Not a word.
  r = messageIn(state, 1, { t: "guess", n: 1, word: "crxne" }, at, connected);
  assert.deepEqual(sent(r.effects, "reject")[0], { t: "reject", n: 1, reason: "not_a_word" });
  state = r.state as RoomState;

  // The wrong length is refused before the list is consulted.
  r = messageIn(state, 1, { t: "guess", n: 1, word: "cran" }, at, connected);
  assert.deepEqual(sent(r.effects, "reject")[0], { t: "reject", n: 1, reason: "wrong_length" });
  state = r.state as RoomState;

  // A good guess is scored, and the others hear of it, colours only.
  r = messageIn(state, 1, { t: "guess", n: 1, word: "slate" }, at, connected);
  const marks = sent(r.effects, "marks");
  assert.equal(marks.length, 1);
  assert.deepEqual((marks[0] as { marks: number[] }).marks, score("slate", WORD));
  const progress = sent(r.effects, "progress");
  assert.equal(progress.length, 1);
  const progressEffect = ofKind(r.effects, "broadcast").find(
    (e) => (e as { msg: { t: string } }).msg.t === "progress",
  ) as { except: number } | undefined;
  assert.equal(progressEffect?.except, 1);
  state = r.state as RoomState;

  // The same guess number again is a resend: the same marks, no progress,
  // and still one word.
  r = messageIn(state, 1, { t: "guess", n: 1, word: "slate" }, at + 1, connected);
  const resent = sent(r.effects, "marks");
  assert.equal(resent.length, 1);
  assert.deepEqual((resent[0] as { marks: number[] }).marks, score("slate", WORD));
  assert.equal(sent(r.effects, "progress").length, 0);
  const seat = (r.state as RoomState).seats.find((s) => s.seat === 1) as { words: string[] };
  assert.equal(seat.words.length, 1);
});

test("a guess before go is not playing", () => {
  const counted = readied(start(true), true);
  const r = messageIn(counted, 1, { t: "guess", n: 1, word: "slate" }, T + 10, [1, 2]);
  assert.deepEqual(sent(r.effects, "reject")[0], { t: "reject", n: 1, reason: "not_playing" });
});

// solvingSeatGone takes a playing room, solves with seat 1, and returns both.
function seat1Solves(mark: number) {
  const round = playing(true);
  const first = messageIn(round, 1, { t: "guess", n: 1, word: "slate" }, T + COUNTDOWN_MS + 1000, [1, 2]);
  const solved = messageIn(first.state as RoomState, 1, { t: "guess", n: 2, word: WORD },
    T + COUNTDOWN_MS + mark, [1, 2]);
  return solved;
}

test("solving finishes the seat", () => {
  const r = seat1Solves(2000);
  const done = sent(r.effects, "done")[0] as { seat: number; solved: boolean; guesses: number; elapsedMs: number };
  assert.equal(done.seat, 1);
  assert.equal(done.solved, true);
  assert.equal(done.guesses, 2);
  assert.equal(done.elapsedMs, T + COUNTDOWN_MS + 2000 - GO_AT);
  assert.equal((r.state as RoomState).phase, "playing", "seat 2 is still playing");
});

test("the last finish ends the round", () => {
  const round = playing(true);
  const solved = messageIn(round, 1, { t: "guess", n: 1, word: WORD }, T + COUNTDOWN_MS + 500, [1, 2]);
  let state = solved.state as RoomState;
  const list = ["slate", "adieu", "pious", "trope", "lymph"];
  for (let i = 0; i < list.length; i++) {
    const r = messageIn(state, 2, { t: "guess", n: i + 1, word: list[i] }, T + COUNTDOWN_MS + 10_000 + i, [1, 2]);
    state = r.state as RoomState;
  }
  const r = messageIn(state, 2, { t: "guess", n: 6, word: "ghost" }, T + COUNTDOWN_MS + 20_000, [1, 2]);
  const dones = sent(r.effects, "done");
  assert.equal(dones.length, 1);
  const done = dones[0] as { seat: number; solved: boolean };
  assert.equal(done.seat, 2);
  assert.equal(done.solved, false);
  const results = sent(r.effects, "results");
  assert.equal(results.length, 1);
  const result = results[0] as { word: string; ranking: { seat: number }[] };
  assert.equal(result.word, WORD);
  assert.deepEqual(result.ranking.map((e) => e.seat), [1, 2]);
  assert.equal((r.state as RoomState).phase, "results");
});

test("time runs out", () => {
  const round = playing(true);
  const r = handle(round, { type: "alarm" }, ctx(ENDS_AT, [1, 2]));
  const dones = sent(r.effects, "done");
  assert.equal(dones.length, 2, "both unfinished seats are finished");
  for (const done of dones) {
    assert.equal((done as { elapsedMs: number }).elapsedMs, 600000);
    assert.equal((done as { solved: boolean }).solved, false);
  }
  assert.equal(sent(r.effects, "results").length, 1);
  assert.equal((r.state as RoomState).phase, "results");
});

test("leaving mid-round counts as not solved", () => {
  const round = playing(true);
  const r = messageIn(round, 2, { t: "leave" }, T + COUNTDOWN_MS + 50_000, [1, 2]);
  const done = sent(r.effects, "done")[0] as { seat: number; solved: boolean; elapsedMs: number };
  assert.equal(done.seat, 2);
  assert.equal(done.solved, false);
  assert.equal(done.elapsedMs, 50_000);
  assert.deepEqual(sent(r.effects, "left")[0], { t: "left", seat: 2 });
  assert.deepEqual(ofKind(r.effects, "close")[0], { kind: "close", seat: 2, code: 1000, reason: "left" });
  const after = r.state as RoomState;
  assert.equal((after.seats.find((s) => s.seat === 2) as { removed: boolean }).removed, true);

  // Its key is refused from then on.
  const back = helloIn(after, "", KEY2.key, "h2", KEY2, T, []);
  assert.deepEqual(sent(back.effects, "refuse")[0], { t: "refuse", reason: "left" });
});

test("the host leaving passes the host on", () => {
  const state = start(true);
  const r = messageIn(state, 1, { t: "leave" }, T + 1, []);
  const hosts = sent(r.effects, "host");
  assert.equal(hosts.length, 1);
  assert.deepEqual(hosts[0], { t: "host", seat: 2 });
  assert.equal((r.state as RoomState).host, 2);
  assert.equal(sent(r.effects, "countdown").length, 0, "an unready room does not start itself");
});

test("rematch goes back to the lobby", () => {
  const round = playing(true);
  const solved = messageIn(round, 1, { t: "guess", n: 1, word: WORD }, T + COUNTDOWN_MS + 500, [1, 2]);
  const out = messageIn(solved.state as RoomState, 2, { t: "guess", n: 1, word: "slate" },
    T + COUNTDOWN_MS + 20_000, [1, 2]);
  const over = handle(out.state as RoomState, { type: "alarm" }, ctx(ENDS_AT, [1, 2]));
  const state = over.state as RoomState;
  assert.equal(state.phase, "results");

  const r = messageIn(state, 2, { t: "rematch" }, ENDS_AT + 1, [1, 2]);
  assert.deepEqual(sent(r.effects, "lobby")[0], { t: "lobby", round: 2 });
  const after = r.state as RoomState;
  assert.equal(after.phase, "lobby");
  assert.equal(after.round, 2);
  for (const seat of after.seats) {
    assert.equal(seat.ready, false);
    assert.equal(seat.inRound, false);
    assert.deepEqual(seat.words, []);
    assert.deepEqual(seat.marks, []);
  }
});

test("an empty lobby is deleted after the grace time", () => {
  const state = start(true);
  const r = handle(state, { type: "disconnect", seat: 1 }, ctx(T + 5, []));
  const after = r.state as RoomState;
  assert.equal(after.deleteAt, T + 5 + GRACE_MS);

  const alarm = handle(after, { type: "alarm" }, ctx(T + 5 + GRACE_MS, []));
  assert.ok(ofKind(alarm.effects, "delete").length === 1);
  assert.equal(alarm.state, null);
});

test("a finished room stays ten minutes", () => {
  const round = playing(true);
  const r = handle(round, { type: "alarm" }, ctx(ENDS_AT, []));
  const after = r.state as RoomState;
  assert.equal(after.phase, "results");
  assert.equal(after.deleteAt, ENDS_AT + KEEP_AFTER_ROUND_MS);
});

test("a rejoin cancels the deletion", () => {
  const round = playing(true);
  const finished = handle(round, { type: "alarm" }, ctx(ENDS_AT, []));
  const r = helloIn(finished.state as RoomState, "nick", KEY1.key, "h1", KEY1, ENDS_AT + 10, []);
  assert.equal((r.state as RoomState).deleteAt, 0);
  assert.equal(sent(r.effects, "welcome").length, 1);
});

test("presence is sent only when the last socket goes", () => {
  const state = start(true);
  const withAnother = handle(state, { type: "disconnect", seat: 2 }, ctx(T + 1, [1, 2]));
  assert.equal(sent(withAnother.effects, "presence").length, 0);

  const last = handle(state, { type: "disconnect", seat: 2 }, ctx(T + 2, [1]));
  const presence = sent(last.effects, "presence");
  assert.equal(presence.length, 1);
  assert.deepEqual(presence[0], { t: "presence", seat: 2, connected: false });
});

// ---- async rooms ----

test("an async room picks its word at creation", () => {
  const state = startAsync();
  assert.equal(state.answer, WORD);
  assert.equal(state.phase, "open");
  assert.equal(state.expiresAt, T + ASYNC_LIFETIME_MS);
  assert.equal(state.host, 0);
});

test("async players see no one else's round", () => {
  let state = startAsync();
  const joined = helloIn(state, "sam", "", null, KEY2, T + 1, []);
  state = joined.state as RoomState;
  const begun = messageIn(state, 2, { t: "start" }, T + 2, [2]);
  state = begun.state as RoomState;
  const list = ["slate", "adieu", "pious", "trope", "lymph", "ghost"];
  for (let i = 0; i < list.length; i++) {
    const r = messageIn(state, 2, { t: "guess", n: i + 1, word: list[i] }, T + 3 + i, [2]);
    state = r.state as RoomState;
  }

  const back = helloIn(state, "nick", KEY1.key, "h1", KEY1, T + 100, []);
  const welcome = sent(back.effects, "welcome")[0] as unknown as {
    room: { players: { seat: number; rows: unknown[]; guesses: number; done: boolean }[] };
  };
  const sam = welcome.room.players.find((p) => p.seat === 2) as { rows: unknown[]; guesses: number; done: boolean };
  assert.deepEqual(sam.rows, []);
  assert.equal(sam.guesses, 0);
  assert.equal(sam.done, true);
});

test("starting runs your own clock", () => {
  const state = startAsync();
  const first = messageIn(state, 1, { t: "start" }, T + 1, [1]);
  const gos = sent(first.effects, "go");
  assert.equal(gos.length, 1);
  const go = gos[0] as { now: number; endsAt: number; round: number };
  assert.equal(go.now, T + 1);
  assert.equal(go.endsAt, T + 1 + ROUND_MS);
  assert.equal(go.round, 1);
  const you = (first.state as RoomState).seats.find((s) => s.seat === 1) as {
    startedAt: number; endsAt: number; inRound: boolean;
  };
  assert.equal(you.startedAt, T + 1);
  assert.equal(you.endsAt, T + 1 + ROUND_MS);
  assert.equal(you.inRound, true);

  const second = messageIn(first.state as RoomState, 1, { t: "start" }, T + 2, [1]);
  assert.equal(sent(second.effects, "go").length, 0);
  assert.deepEqual(sent(second.effects, "error")[0], { t: "error", reason: "not_allowed" });
});

test("finishing records and answers", () => {
  const state = startAsync();
  const begun = messageIn(state, 1, { t: "start" }, T + 1, [1]);
  const r = messageIn(begun.state as RoomState, 1, { t: "guess", n: 1, word: WORD }, T + 600, [1]);
  const done = sent(r.effects, "done")[0] as { solved: boolean; guesses: number; elapsedMs: number };
  assert.equal(done.solved, true);
  assert.equal(done.guesses, 1);
  assert.equal(done.elapsedMs, 599);

  const records = ofKind(r.effects, "record");
  assert.equal(records.length, 1);
  const row = (records[0] as unknown as { row: { solved: boolean; guesses: number; rows: number[][]; seat: number } }).row;
  assert.equal(row.solved, true);
  assert.equal(row.guesses, 1);
  assert.deepEqual(row.rows, [score(WORD, WORD)]);
  assert.equal(row.seat, 1);

  const results = sent(r.effects, "results");
  assert.equal(results.length, 1);
  const result = results[0] as { word: string; ranking: { seat: number }[] };
  assert.equal(result.word, WORD);
  assert.deepEqual(result.ranking.map((e) => e.seat), [1]);
});

test("an async seat runs out of time", () => {
  let state = startAsync();
  const joined = helloIn(state, "sam", "", null, KEY2, T + 1, []);
  state = joined.state as RoomState;
  const begun = messageIn(state, 2, { t: "start" }, T + 2, [2]);
  state = begun.state as RoomState;
  const endsAt = T + 2 + ROUND_MS;

  const r = handle(state, { type: "alarm" }, ctx(endsAt, []));
  const records = ofKind(r.effects, "record");
  assert.equal(records.length, 1);
  const row = (records[0] as unknown as { row: { solved: boolean; elapsedMs: number } }).row;
  assert.equal(row.solved, false);
  assert.equal(row.elapsedMs, ROUND_MS as number);
  assert.equal(sent(r.effects, "done").length, 0, "nobody is connected, so nothing is sent");
  assert.equal(sent(r.effects, "results").length, 0);
});

test("the fifty-first player is refused", () => {
  let state = startAsync();
  for (let seat = 2; seat <= 50; seat++) {
    const fresh = { key: String(seat).padStart(32, "0"), keyHash: "h" + seat };
    const r = helloIn(state, "p" + seat, "", null, fresh, T, []);
    state = r.state as RoomState;
  }
  const r = helloIn(state, "p51", "", null, { key: "f".repeat(32), keyHash: "h51" }, T, []);
  assert.deepEqual(sent(r.effects, "refuse")[0], { t: "refuse", reason: "full" });
});

test("the room expires", () => {
  const state = startAsync();
  const r = handle(state, { type: "alarm" }, ctx(T + ASYNC_LIFETIME_MS, []));
  assert.deepEqual(ofKind(r.effects, "closeAll")[0], { kind: "closeAll", code: 4004, reason: "room closed" });
  assert.equal(ofKind(r.effects, "delete").length, 1);
  assert.equal(r.state, null);
});

test("async rooms never broadcast", () => {
  let state = startAsync();
  const produced: Effect[] = [];
  const joined = helloIn(state, "sam", "", null, KEY2, T + 1, []);
  state = joined.state as RoomState;
  produced.push(...joined.effects);
  const begun = messageIn(state, 2, { t: "start" }, T + 2, [2]);
  state = begun.state as RoomState;
  produced.push(...begun.effects);
  const list = ["slate", "adieu", WORD];
  for (let i = 0; i < list.length; i++) {
    const r = messageIn(state, 2, { t: "guess", n: i + 1, word: list[i] }, T + 3 + i, [2]);
    state = r.state as RoomState;
    produced.push(...r.effects);
  }
  assert.ok(produced.length > 0, "the walk produced effects");
  assert.ok(!produced.some((e) => e.kind === "broadcast"), "no effect of kind broadcast");
});

// ---- invariants ----

test("handle never changes its input", () => {
  const state = start(true);
  const before = structuredClone(state);
  messageIn(state, 1, { t: "ready", ready: true }, T + 1, [1]);
  helloIn(state, "sam", "", null, KEY2, T + 2, [1]);
  handle(state, { type: "disconnect", seat: 2 }, ctx(T + 3, [1]));
  assert.deepEqual(state, before);
});

test("nextDeadline picks the earliest", () => {
  assert.equal(nextDeadline(null), null);

  const counted = readied(start(true), true);
  assert.equal(nextDeadline(counted), (counted as RoomState).goAt);

  const round = playing(true);
  const withDeletion = structuredClone(round) as RoomState;
  withDeletion.deleteAt = withDeletion.endsAt - 1000;
  assert.equal(nextDeadline(withDeletion), withDeletion.endsAt - 1000, "deleteAt lower than endsAt wins");

  const begun = messageIn(startAsync(), 1, { t: "start" }, T + 1, [1]);
  const seat = (begun.state as RoomState).seats.find((s) => s.seat === 1) as { endsAt: number };
  assert.equal(nextDeadline(begun.state as RoomState), Math.min(T + ASYNC_LIFETIME_MS, seat.endsAt));
});
