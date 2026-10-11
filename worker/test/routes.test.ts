import { test } from "node:test";
import assert from "node:assert/strict";
import { match } from "../src/routes.ts";

test("match finds the routes", () => {
  assert.deepEqual(match("/api/v1/status"),
    { route: { name: "status" }, methods: ["GET"] });
  assert.deepEqual(match("/api/v1/daily/2026-10-11/5"),
    { route: { name: "daily", day: "2026-10-11", length: "5" }, methods: ["GET", "POST"] });
  assert.deepEqual(match("/api/v1/rooms"),
    { route: { name: "rooms" }, methods: ["POST"] });
  assert.deepEqual(match("/api/v1/rooms/K7M2QX/socket"),
    { route: { name: "room_socket", code: "K7M2QX" }, methods: ["GET"] });
  assert.deepEqual(match("/api/v1/rooms/K7M2QX/results"),
    { route: { name: "room_results", code: "K7M2QX" }, methods: ["GET"] });

  for (const missing of [
    "/api/v1/status/", // a trailing slash is not accepted
    "/api/v1/daily/2026-10-11",
    "/api/v1/daily/a/b/c",
    "/api/v1/rooms/K7M2QX",
    "/api/v1/rooms/K7M2QX/x",
    "/api/v2/status",
    "/",
  ]) {
    assert.deepEqual(match(missing), { route: { name: "not_found" }, methods: [] }, missing);
  }
});
