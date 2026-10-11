#!/usr/bin/env node
// Drive the Worker's /api end to end, locally against `wrangler dev`, against
// staging, or read-only against production.
//
//   node scripts/smoke-api.mjs <origin> [--read-only] [--rooms]
//
// <origin> is like http://127.0.0.1:8787 or https://surmise.nxck.dev; a
// trailing / is allowed and removed. Every request names itself in
// X-Surmise-Client. Each check prints "ok  <description>"; the first failure
// prints "FAIL <description>: <expected> / <back>" and exits 1.

// The same value CI and the deploy job greet the Worker with. 0.0.0-smoke is
// older than any MIN_CLIENT this account is likely to set, so the smoke test
// also exercises the version check's accepting path.
const CLIENT_HEADER = "X-Surmise-Client";
const CLIENT = "surmise/0.0.0-smoke (linux/amd64)";
const CODE_PATTERN = /^[0-9A-HJKMNP-TV-Z]{6}$/;
const KEY_PATTERN = /^[0-9a-f]{32}$/;

let origin = null;
let readOnly = false;
let rooms = false;
for (const arg of process.argv.slice(2)) {
  if (arg === "--read-only") readOnly = true;
  else if (arg === "--rooms") rooms = true;
  else if (origin === null) origin = arg;
}
if (origin === null) {
  console.error("usage: smoke-api.mjs <origin> [--read-only] [--rooms]");
  process.exit(2);
}
origin = origin.replace(/\/+$/, "");

function fail(desc, want, got) {
  console.log(`FAIL ${desc}: ${want} / ${got}`);
  process.exit(1);
}

function ok(desc) {
  console.log(`ok  ${desc}`);
}

async function call(method, path, body, opts = {}) {
  const headers = {};
  if (!opts.noHeader) headers[CLIENT_HEADER] = CLIENT;
  const init = { method, headers };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  return fetch(origin + path, init);
}

function isStatus(res, desc, want) {
  if (res.status !== want) fail(desc, `status ${want}`, `status ${res.status}`);
}

function expect(condition, desc, want, got) {
  if (!condition) fail(desc, want, got);
}

// 1. The server offers the daily feature and a string message.
//
// A deploy's static files and its script reach the edge separately, so the
// site can already serve the new boot.js while some edges still answer /api
// from the version before — the first production deploy of the script hit
// exactly that gap. Wait up to two minutes for the status, as the site check
// waits for boot.js, before anything else is asserted.
let res;
for (let attempt = 1; ; attempt++) {
  res = await call("GET", "/api/v1/status");
  if (res.status === 200 || attempt === 12) break;
  res.body?.cancel?.();
  await new Promise((resolve) => setTimeout(resolve, 10_000));
}
isStatus(res, "GET status answers 200", 200);
const status = JSON.parse(await res.text());
expect(status.features?.daily === true, "GET status offers the daily feature",
  "features.daily true", JSON.stringify(status));
expect(typeof status.message === "string", "GET status carries a string message",
  "string", String(status.message));
ok("GET status answers with the daily flag and a message");

// 2. Without the client header there is no API.
res = await call("GET", "/api/v1/status", undefined, { noHeader: true });
expect(res.status === 400, "GET status without the client header",
  "400", String(res.status));
res.body?.cancel?.();
ok("GET status without the client header is turned away");

// 3. An unknown route is not found.
res = await call("GET", "/api/v1/nope");
expect(res.status === 404, "GET an unknown route", "404", String(res.status));
res.body?.cancel?.();
ok("GET an unknown route is not found");

if (readOnly) {
  process.exit(0);
}

// Writes below this point. Staging and local get the full walk; production is
// checked --read-only, so a release never adds a fake player to a real day.

// 4. A solved 5-letter round posts cleanly.
const today = new Date().toISOString().slice(0, 10);
res = await call("POST", `/api/v1/daily/${today}/5`, { solved: true, guesses: 3 });
expect(res.status === 204, `POST the daily for ${today}`, "204", String(res.status));
res.body?.cancel?.();
ok("POST the daily is accepted and answers no content");

// 5. The count that came back knows about it.
res = await call("GET", `/api/v1/daily/${today}/5`);
isStatus(res, "GET the daily count answers 200", 200);
const count = JSON.parse(await res.text());
expect(count.played >= 1, "the count has at least one player",
  "played >= 1", `played ${JSON.stringify(count)}`);
expect(count.solved >= 1, "the count has at least one solver",
  "solved >= 1", `solved ${count.solved}`);
expect(Array.isArray(count.distribution) && count.distribution.length === 6,
  "the distribution covers 5-letter rounds", "length 6", `length ${count.distribution?.length}`);
expect(count.distribution[2] >= 1, "the distribution counts the smoke post",
  ">= 1", String(count.distribution[2]));
ok("GET the daily count shows the post we just made");

// 6. A guess count no 5-letter round can have is refused.
res = await call("POST", `/api/v1/daily/${today}/5`, { solved: true, guesses: 9 });
expect(res.status === 400, "POST an impossible guess count", "400", String(res.status));
res.body?.cancel?.();
ok("POST an impossible guess count is refused");

// 7. A day outside the post window is refused.
res = await call("POST", "/api/v1/daily/1999-01-01/5", { solved: true, guesses: 3 });
expect(res.status === 400, "POST a day that is not today", "400", String(res.status));
res.body?.cancel?.();
ok("POST a day that is not today is refused");

// 8. Anything but GET and POST is a method error, with the two listed.
res = await call("PUT", `/api/v1/daily/${today}/5`, { solved: true, guesses: 3 });
expect(res.status === 405, "PUT the daily", "405", String(res.status));
res.body?.cancel?.();
ok("PUT the daily is a method error");

if (!rooms) {
  console.log("done");
  process.exit(0);
}

// ---- rooms. Node 24 has a global WebSocket. Each check runs the real
// protocol end to end against the Worker's Durable Object.

const GUESSES = ["crane", "slate", "adieu", "pious", "trope", "lymph"];

// open(code) connects and returns { send, next, close, closed }. next resolves
// with the first unread message of type t, waiting up to ms; messages of other
// types are kept in order for later next() calls. The socket URL is origin
// with http→ws / https→wss, plus /api/v1/rooms/<code>/socket.
async function open(code) {
  const url = origin.replace(/^http/, "ws") + "/api/v1/rooms/" + code + "/socket";
  const ws = new WebSocket(url);
  const queue = [];
  const waiters = [];
  ws.addEventListener("message", (ev) => {
    let msg;
    try {
      msg = JSON.parse(String(ev.data));
    } catch {
      return;
    }
    const at = waiters.findIndex((w) => w.t === msg.t);
    if (at >= 0) {
      const [w] = waiters.splice(at, 1);
      w.resolve(msg);
    } else {
      queue.push(msg);
    }
  });
  await new Promise((resolve, reject) => {
    if (ws.readyState === 1) {
      resolve();
      return;
    }
    ws.addEventListener("open", resolve, { once: true });
    ws.addEventListener("error", () => reject(new Error("socket failed to open")), { once: true });
  });
  return {
    ws,
    send(obj) {
      ws.send(JSON.stringify(obj));
    },
    close() {
      ws.close(1000, "closing");
    },
    closed: new Promise((resolve) => {
      ws.addEventListener("close", (ev) => resolve(ev.code), { once: true });
    }),
    async next(t, ms = 8000) {
      const buffered = queue.findIndex((m) => m.t === t);
      if (buffered >= 0) return queue.splice(buffered, 1)[0];
      return new Promise((resolve) => {
        const timer = setTimeout(() => {
          const at = waiters.findIndex((w) => w === waiter);
          if (at >= 0) waiters.splice(at, 1);
          resolve(null);
        }, ms);
        const waiter = { t, resolve(msg) {
          clearTimeout(timer);
          resolve(msg);
        } };
        waiters.push(waiter);
      });
    },
  };
}

// play(sock, n0) guesses the list in order, waiting for each answer, and stops
// after the guess whose marks are all 2, or after six. The return is the
// number of guesses used.
async function play(sock, n0 = 1) {
  for (let i = 0; i < GUESSES.length; i++) {
    sock.send({ t: "guess", n: n0 + i, word: GUESSES[i] });
    const marks = await sock.next("marks");
    if (marks === null || !Array.isArray(marks.marks) || marks.n !== n0 + i) {
      fail("play got marks for guess " + (n0 + i), "marks with n " + (n0 + i), JSON.stringify(marks));
    }
    if (marks.marks.every((m) => m === 2)) return n0 + i;
  }
  return 6;
}

// helloSend is a well-formed hello carrying the seat key, or "" to join fresh.
function helloSend(key, name) {
  return { t: "hello", v: 1, client: CLIENT, name, key };
}

// Rooms. 1. A live room is made.
res = await call("POST", "/api/v1/rooms", { mode: "live", length: 5, autoStart: true });
isStatus(res, "POST a live room", 201);
const liveRoom = JSON.parse(await res.text());
expect(CODE_PATTERN.test(liveRoom.code), "the room code is six channel letters", "code pattern", liveRoom.code);
expect(KEY_PATTERN.test(liveRoom.key), "the creator gets a 32-hex seat key", "key pattern", JSON.stringify(liveRoom.key));
expect(liveRoom.seat === 1, "the creator is seat 1", "seat 1", String(liveRoom.seat));
ok("POST a live room answers code, key and seat");

// 2. A joins with the creator's key.
const a = await open(liveRoom.code);
a.send(helloSend(liveRoom.key, "smoke a"));
const aWelcome = await a.next("welcome");
expect(aWelcome !== null && aWelcome.seat === 1, "A's welcome names seat 1", "seat 1", JSON.stringify(aWelcome));
expect(aWelcome.room.phase === "lobby", "A arrives on the lobby", "lobby", aWelcome.room.phase);
ok("A's hello is answered with a welcome on the lobby");

// 3. B joins fresh.
const b = await open(liveRoom.code);
b.send(helloSend("", "smoke b"));
const bWelcome = await b.next("welcome");
expect(bWelcome !== null && bWelcome.seat === 2, "B's welcome names seat 2", "seat 2", JSON.stringify(bWelcome));
const joined = await a.next("joined");
expect(joined !== null && joined.player.seat === 2, "A hears of B", "joined seat 2", JSON.stringify(joined));
ok("B joins and A hears of it");

// 4. Both ready up; the room starts by itself.
b.send({ t: "ready", ready: true }); // B readies first: not everyone is ready yet
a.send({ t: "ready", ready: true });
const aCountdown = await a.next("countdown");
const bCountdown = await b.next("countdown");
expect(aCountdown !== null && bCountdown !== null, "both hears the countdown", "countdown", JSON.stringify(aCountdown));
const aGo = await a.next("go", 10_000);
const bGo = await b.next("go", 10_000);
expect(aGo !== null && bGo !== null, "the countdown is answered by go", "go", JSON.stringify(aGo));
ok("two ready players start the round");

// 5. Both play; the round ends with a ranking.
await play(a);
await play(b);
const aResults = await a.next("results");
const bResults = await b.next("results");
expect(aResults !== null && /^[a-z]{5}$/.test(aResults.word), "A sees the word", "a 5-letter word", JSON.stringify(aResults));
expect(bResults !== null && Array.isArray(bResults.ranking) && bResults.ranking.length === 2,
  "B sees a ranking of two", "ranking.length 2", JSON.stringify(bResults));
ok("both players see the round's result");

// 6. Rematch.
b.send({ t: "rematch" });
const aLobby = await a.next("lobby");
expect(aLobby !== null && aLobby.round === 2, "the rematch is a new lobby", "lobby round 2", JSON.stringify(aLobby));
ok("rematch returns to the lobby for round 2");

// 7. A leaves; its seat's key is retired and the host seat moves.
a.send({ t: "leave" });
const aCloseCode = await a.closed;
expect(aCloseCode === 1000, "A's socket closes normally", "close 1000", String(aCloseCode));
const bLeft = await b.next("left");
expect(bLeft !== null && bLeft.seat === 1, "B hears of A's seat", "left seat 1", JSON.stringify(bLeft));
const bHost = await b.next("host");
expect(bHost !== null && bHost.seat === 2, "B is the host now", "host 2", JSON.stringify(bHost));
b.close();
ok("A leaves and B inherits the host seat");

// 8. An any-time room, played alone.
res = await call("POST", "/api/v1/rooms", { mode: "async", length: 5 });
isStatus(res, "POST an async room", 201);
const asyncRoom = JSON.parse(await res.text());
const c = await open(asyncRoom.code);
c.send(helloSend(asyncRoom.key, "smoke c"));
const cWelcome = await c.next("welcome");
expect(cWelcome !== null && cWelcome.room.phase === "open", "C arrives on the open view", "open", JSON.stringify(cWelcome));
c.send({ t: "start" });
const cGo = await c.next("go");
expect(cGo !== null, "C's start is answered by go", "go", JSON.stringify(cGo));
await play(c);
const cDone = await c.next("done");
expect(cDone !== null, "C hears its own done", "done", JSON.stringify(cDone));
const cResults = await c.next("results");
expect(cResults !== null && Array.isArray(cResults.ranking) && cResults.ranking.length === 1,
  "C's result ranks one player", "ranking.length 1", JSON.stringify(cResults));
ok("an any-time room plays on the player's own clock");

// 9. The results endpoint answers the finished seat.
const resultsPath = "/api/v1/rooms/" + asyncRoom.code + "/results";
res = await fetch(origin + resultsPath, { headers: { [CLIENT_HEADER]: CLIENT, Authorization: "Bearer " + asyncRoom.key } });
isStatus(res, "GET the async results", 200);
const theirResults = JSON.parse(await res.text());
expect(theirResults.word === cResults.word, "the endpoint shows the same word", cResults.word, theirResults.word);
expect(Array.isArray(theirResults.ranking) && theirResults.ranking.length === 1, "the endpoint ranks one seat", "ranking.length 1", JSON.stringify(theirResults.ranking));
ok("GET the room results answers the finished seat");
res = await fetch(origin + resultsPath, { headers: { [CLIENT_HEADER]: CLIENT, Authorization: "Bearer " + "f".repeat(32) } });
isStatus(res, "GET the results with a stranger's key", 403);
res.body?.cancel?.();
ok("GET the results with a stranger's key is refused");
res = await fetch(origin + resultsPath, { headers: { [CLIENT_HEADER]: CLIENT } });
isStatus(res, "GET the results with no key", 401);
res.body?.cancel?.();
ok("GET the results with no key is unauthorized");

// 10. A socket to a room that does not exist is refused.
const missing = await open("ZZZZZZ");
missing.send(helloSend("", "smoke missing"));
const refused = await missing.next("refuse");
expect(refused !== null && refused.reason === "no_room", "the no_room refusal", "no_room", JSON.stringify(refused));
await missing.closed;
ok("a socket to a room that does not exist is refused");

// 11. A gone message is the version check over the socket.
const wrong = await open(liveRoom.code);
wrong.send({ t: "hello", v: 2, client: CLIENT, name: "smoke gone", key: "" });
const goneMsg = await wrong.next("gone");
expect(goneMsg !== null && typeof goneMsg.message === "string", "the room refuses version 2", "gone", JSON.stringify(goneMsg));
await wrong.closed;
ok("a hello with the wrong version is turned away");
