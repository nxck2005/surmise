import { test } from "node:test";
import assert from "node:assert/strict";
import { CLIENT_HEADER, isTooOld, parseClient } from "../src/client.ts";

test("parseClient accepts a real header", () => {
  assert.deepEqual(parseClient("surmise/0.9.0 (linux/amd64)"),
    { name: "surmise", version: "0.9.0", platform: "linux/amd64" });
  assert.notEqual(parseClient("surmise/dev (js/wasm)"), null);
  assert.notEqual(parseClient("surmise/v0.9.1-0.20261011-abcdef (darwin/arm64)"), null);
  assert.equal(CLIENT_HEADER, "X-Surmise-Client");
});

test("parseClient refuses anything else", () => {
  const refusals = [
    null,
    "",
    "surmise/0.9.0",
    "Surmise/0.9.0 (linux/amd64)",
    "surmise/0.9.0 (linux/amd64) extra",
    "surmise/0 9 (linux/amd64)",
  ];
  for (const value of refusals) {
    assert.equal(parseClient(value), null, `accepted ${String(value)}`);
  }
});

test("isTooOld compares versions", () => {
  assert.equal(isTooOld("0.9.0", ""), false);
  assert.equal(isTooOld("0.8.9", "0.9.0"), true);
  assert.equal(isTooOld("0.9.0", "0.9.0"), false);
  assert.equal(isTooOld("0.10.0", "0.9.5"), false);
  assert.equal(isTooOld("v0.9.0", "0.9.1"), true);
  assert.equal(isTooOld("dev", "0.9.0"), false);
  assert.equal(isTooOld("0.9.0", "junk"), false);
});
