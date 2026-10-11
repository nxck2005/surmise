import { test } from "node:test";
import assert from "node:assert/strict";
import { readJSON } from "../src/http.ts";

function post(body: string | ReadableStream<Uint8Array>, headers: Record<string, string> = {}): Request {
  return new Request("http://local/api/v1/daily/2026-10-11/5", {
    method: "POST",
    body,
    headers,
    // A stream body needs this in Node; it is ignored for a string.
    duplex: "half",
  } as RequestInit);
}

// stream sends the body in chunks with no Content-Length, the way a chunked
// upload arrives.
function stream(text: string, chunk = 1000): ReadableStream<Uint8Array> {
  const bytes = new TextEncoder().encode(text);
  let at = 0;
  return new ReadableStream({
    pull(controller) {
      if (at >= bytes.length) {
        controller.close();
        return;
      }
      controller.enqueue(bytes.slice(at, at + chunk));
      at += chunk;
    },
  });
}

test("readJSON accepts a small object", async () => {
  const got = await readJSON(post('{"solved":true,"guesses":3}'), 4096);
  assert.deepEqual(got.ok && got.value, { solved: true, guesses: 3 });
});

test("readJSON refuses a body past the cap by its Content-Length", async () => {
  const got = await readJSON(post("x".repeat(10), { "Content-Length": "5000" }), 4096);
  assert.equal(!got.ok && got.response.status, 413);
});

test("readJSON refuses a chunked body past the cap while it arrives", async () => {
  const got = await readJSON(post(stream("a".repeat(100_000))), 4096);
  assert.equal(!got.ok && got.response.status, 413);
});

test("readJSON counts bytes, not characters", async () => {
  // 1500 three-byte characters: 4500 bytes, under 4096 characters.
  const got = await readJSON(post(stream(JSON.stringify({ s: "€".repeat(1500) }))), 4096);
  assert.equal(!got.ok && got.response.status, 413);
});

test("readJSON refuses what is not a JSON object", async () => {
  for (const body of ["x", "[1]", "null", "3"]) {
    const got = await readJSON(post(body), 4096);
    assert.equal(!got.ok && got.response.status, 400, body);
  }
});
