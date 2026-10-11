// The rooms HTTP routes: make a room, open its socket, read an async room's
// results. Workers-only: every route talks to a binding or D1.
//
// The socket route is the one route without the client header, because a
// browser cannot send headers on a WebSocket. Its version check happens in
// hello instead.

import { KEY_PATTERN, isCode, randomCode, sha256hex } from "./codes.ts";
import { flag } from "./flags.ts";
import { error, json, readJSON } from "./http.ts";
import { allowed } from "./limits.ts";
import { rank } from "./room/rank.ts";
import type { RankEntry } from "./room/protocol.ts";

interface CreateBody {
  mode: unknown;
  length: unknown;
  autoStart: unknown;
}

export async function createRoomRoute(request: Request, env: Env): Promise<Response> {
  if (!(await allowed(env.RL_WRITE, request))) return error(429, "rate_limited");
  const body = await readJSON(request, 4096);
  if (!body.ok) return body.response;
  const opts = body.value as CreateBody;

  if (opts.mode !== "live" && opts.mode !== "async") {
    return error(400, "bad_request", "mode must be live or async");
  }
  if (opts.length !== 4 && opts.length !== 5 && opts.length !== 6) {
    return error(400, "bad_request", "length must be 4, 5 or 6");
  }
  let autoStart = false;
  if (opts.autoStart !== undefined) {
    if (typeof opts.autoStart !== "boolean") {
      return error(400, "bad_request", "autoStart must be true or false");
    }
    autoStart = opts.autoStart;
  }
  // An any-time room has no host and nothing to wait for.
  if (opts.mode === "async") autoStart = false;

  // Codes are drawn until one is free. Five tries is the cap: a million-code
  // space means the next try is worse odds than the deploy being unlucky.
  for (let attempt = 0; attempt < 5; attempt++) {
    const code = randomCode();
    const stub = env.ROOMS.get(env.ROOMS.idFromName(code));
    const res = await stub.fetch(new Request("https://room/init", {
      method: "POST",
      body: JSON.stringify({ code, mode: opts.mode, length: opts.length, autoStart }),
    }));
    if (res.status === 201) {
      const made = await res.json() as { key: string };
      return json(201, { code, key: made.key, seat: 1 });
    }
    await cancelBody(res);
    if (res.status !== 409) break;
  }
  return error(500, "server");
}

export async function socketRoute(request: Request, env: Env, code: string): Promise<Response> {
  if (!isCode(code)) return error(404, "not_found");
  if (!flag(env.FEATURE_ROOMS)) return error(503, "unavailable");
  if (!(await allowed(env.RL_JOIN, request))) return error(429, "rate_limited");
  if ((request.headers.get("Upgrade") ?? "").toLowerCase() !== "websocket") {
    return error(426, "upgrade_required");
  }
  // Everything from here is the Durable Object for this code.
  return env.ROOMS.get(env.ROOMS.idFromName(code)).fetch(request);
}

export async function resultsRoute(request: Request, env: Env, code: string): Promise<Response> {
  if (!isCode(code)) return error(404, "not_found");
  if (!(await allowed(env.RL_READ, request))) return error(429, "rate_limited");

  const authorization = request.headers.get("Authorization") ?? "";
  if (!authorization.startsWith("Bearer ")) return error(401, "unauthorized");
  const key = authorization.slice("Bearer ".length);
  if (!KEY_PATTERN.test(key)) return error(401, "unauthorized");

  const now = Date.now();
  // The endpoint serves any-time rooms only: a live room has nothing in D1,
  // so this select is its refusal too.
  const room = await env.DB
    .prepare("SELECT answer FROM async_rooms WHERE code = ?1 AND expires_at > ?2")
    .bind(code, now)
    .all<{ answer: string }>();
  if (room.results.length === 0) return error(404, "not_found");

  // The reader must be a finished seat of that room.
  const own = await env.DB
    .prepare("SELECT seat FROM async_results WHERE code = ?1 AND key_hash = ?2")
    .bind(code, await sha256hex(key))
    .all<{ seat: number }>();
  if (own.results.length === 0) return error(403, "forbidden");

  const rows = await env.DB
    .prepare("SELECT seat, name, solved, guesses, elapsed_ms, rows FROM async_results WHERE code = ?1")
    .bind(code)
    .all<{ seat: number; name: string; solved: number; guesses: number; elapsed_ms: number; rows: string }>();
  const entries: RankEntry[] = rows.results.map((r) => ({
    seat: r.seat,
    name: r.name,
    solved: r.solved === 1,
    guesses: r.guesses,
    elapsedMs: r.elapsed_ms,
    rows: JSON.parse(r.rows) as number[][],
  }));
  return json(200, { word: room.results[0].answer, ranking: rank(entries) });
}

async function cancelBody(res: Response): Promise<void> {
  try {
    await res.body?.cancel();
  } catch {
    // A body that is already gone is not a failure.
  }
}
