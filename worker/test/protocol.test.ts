import { test } from "node:test";
import assert from "node:assert/strict";
import { parseClientMessage } from "../src/room/protocol.ts";

test("parse accepts each client message", () => {
  // One valid example of each of the six types; every extra field dropped.
  assert.deepEqual(
    parseClientMessage('{"t":"hello","v":1,"client":"surmise/0.10.0 (linux/amd64)","name":"nick","key":"000000000000000000000000000000ff","extra":1}'),
    { ok: true, msg: { t: "hello", v: 1, client: "surmise/0.10.0 (linux/amd64)", name: "nick", key: "000000000000000000000000000000ff" } },
  );
  assert.deepEqual(
    parseClientMessage('{"t":"ready","ready":true,"extra":9}'),
    { ok: true, msg: { t: "ready", ready: true } },
  );
  assert.deepEqual(
    parseClientMessage('{"t":"start","extra":1}'),
    { ok: true, msg: { t: "start" } },
  );
  assert.deepEqual(
    parseClientMessage('{"t":"guess","n":2,"word":"crane","extra":1}'),
    { ok: true, msg: { t: "guess", n: 2, word: "crane" } },
  );
  assert.deepEqual(
    parseClientMessage('{"t":"rematch","extra":1}'),
    { ok: true, msg: { t: "rematch" } },
  );
  assert.deepEqual(
    parseClientMessage('{"t":"leave","extra":1}'),
    { ok: true, msg: { t: "leave" } },
  );
});

test("parse refuses bad shapes", () => {
  for (const text of ["x", "[]", "null", "3"]) {
    assert.deepEqual(parseClientMessage(text), { ok: false, reason: "not_json" }, text);
  }
  assert.deepEqual(parseClientMessage('{"t":"dance"}'), { ok: false, reason: "unknown_type" });

  const bad = [
    '{"t":"hello","v":"1","client":"surmise/0.10.0 (linux/amd64)","name":"n","key":""}',
    '{"t":"hello","v":1,"client":"surmise/0.10.0 (linux/amd64)","name":"n","key":"abc"}',
    '{"t":"hello","v":1,"client":"","name":"n","key":""}',
    `{"t":"hello","v":1,"client":"surmise/0.10.0 (linux/amd64)","name":"${"y".repeat(65)}","key":""}`,
    '{"t":"guess","n":0,"word":"crane"}',
    '{"t":"guess","n":8,"word":"crane"}',
    '{"t":"guess","n":1.5,"word":"crane"}',
    '{"t":"guess","n":1,"word":""}',
    '{"t":"ready"}',
  ] as const;
  for (const text of bad) {
    assert.deepEqual(parseClientMessage(text), { ok: false, reason: "bad_message" }, text);
  }
});
