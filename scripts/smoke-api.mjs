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
let res = await call("GET", "/api/v1/status");
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

if (rooms) {
  console.log("rooms: not in this build");
}
