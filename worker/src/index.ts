// The entry point. Workers-only: every binding reaches this file through env,
// so the Node tests can load everything under src/ except this file.
import { CLIENT_HEADER, isTooOld, parseClient } from "./client.ts";
import { LENGTHS, bucket, checkGetDay, checkPostDay, cutoffDay, parseLength, parsePost, summarize } from "./daily.ts";
import { flag } from "./flags.ts";
import { error, gone, json, noContent, readJSON } from "./http.ts";
import { allowed } from "./limits.ts";
import { match } from "./routes.ts";

// 405 with the Allow header the rule asks for. Built from the response the
// helpers make, so the headers stay consistent.
function notAllowed(methods: readonly string[]): Response {
  const response = error(405, "method_not_allowed");
  const headers = new Headers(response.headers);
  headers.set("Allow", methods.join(", "));
  return new Response(response.body, { status: response.status, headers });
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    // run_worker_first should never send a non-/api request here; this is the
    // safe fallback to the ordinary static asset fetch it would have been.
    if (!url.pathname.startsWith("/api/")) {
      return env.ASSETS.fetch(request);
    }
    try {
      const client = parseClient(request.headers.get(CLIENT_HEADER));
      if (client === null) {
        return error(400, "bad_request", "missing or malformed X-Surmise-Client header");
      }
      if (isTooOld(client.version, env.MIN_CLIENT)) {
        return gone(env.GONE_MESSAGE);
      }

      const m = match(url.pathname);
      if (m.route.name === "not_found") {
        return error(404, "not_found");
      }
      if (!m.methods.includes(request.method)) {
        return notAllowed(m.methods);
      }

      if (m.route.name === "status") {
        if (!(await allowed(env.RL_READ, request))) {
          return error(429, "rate_limited");
        }
        return json(200, {
          features: { daily: flag(env.FEATURE_DAILY), rooms: flag(env.FEATURE_ROOMS) },
          message: env.STATUS_MESSAGE ?? "",
        });
      }

      // m.route.name === "daily" from here on.
      if (!flag(env.FEATURE_DAILY)) {
        // A switched-off feature is not 410: 410 tells the client to stop
        // using the network altogether.
        return error(503, "unavailable");
      }
      const length = parseLength(m.route.length);
      if (length === null) {
        return error(400, "bad_request", "length must be 4, 5 or 6");
      }
      const day = m.route.day;

      if (request.method === "POST") {
        if (!(await allowed(env.RL_WRITE, request))) {
          return error(429, "rate_limited");
        }
        if (!checkPostDay(day, Date.now())) {
          return error(400, "bad_request", "day is not today");
        }
        // The one body cap (Part 2): a request may not carry more than this.
        const body = await readJSON(request, 4096);
        if (!body.ok) return body.response;
        const post = parsePost(body.value, length);
        if (typeof post === "string") {
          return error(400, "bad_request", post);
        }
        await env.DB.prepare(
          "INSERT INTO daily_counts (day, length, guesses, n) VALUES (?1, ?2, ?3, 1) " +
            "ON CONFLICT (day, length, guesses) DO UPDATE SET n = n + 1",
        ).bind(day, length, bucket(post)).run();
        return noContent();
      }

      // GET.
      if (!(await allowed(env.RL_READ, request))) {
        return error(429, "rate_limited");
      }
      if (!checkGetDay(day, Date.now())) {
        return error(400, "bad_request", "day is out of range");
      }
      const rows = await env.DB
        .prepare("SELECT guesses, n FROM daily_counts WHERE day = ?1 AND length = ?2")
        .bind(day, length)
        .all<{ guesses: number; n: number }>();
      return json(200, summarize(length, rows.results));
    } catch {
      return error(500, "server");
    }
  },

  async scheduled(_controller: ScheduledController, env: Env): Promise<void> {
    await env.DB.prepare("DELETE FROM daily_counts WHERE day < ?1")
      .bind(cutoffDay(Date.now()))
      .run();
  },
} satisfies ExportedHandler<Env>;
