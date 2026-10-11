import { test } from "node:test";
import assert from "node:assert/strict";
import { bucket, checkGetDay, checkPostDay, parsePost, summarize } from "../src/daily.ts";

test("parsePost checks every field", () => {
  assert.deepEqual(parsePost({ solved: true, guesses: 3 }, 5), { solved: true, guesses: 3 });
  assert.deepEqual(parsePost({ solved: false, guesses: 6 }, 5), { solved: false, guesses: 6 });
  assert.equal(parsePost({ solved: "yes", guesses: 3 }, 5), "solved must be true or false");
  assert.equal(parsePost({ solved: true, guesses: 2.5 }, 5), "guesses must be a whole number");
  assert.equal(parsePost({ solved: true, guesses: 0 }, 5), "guesses is out of range");
  assert.equal(parsePost({ solved: true, guesses: 7 }, 5), "guesses is out of range");
  assert.equal(parsePost({ solved: false, guesses: 5 }, 5), "guesses is out of range");
  // Unknown fields are ignored.
  assert.deepEqual(parsePost({ solved: true, guesses: 3, extra: 1 }, 5), { solved: true, guesses: 3 });
});

test("the post window is a day either side", () => {
  const now = Date.UTC(2026, 9, 11, 12);
  assert.equal(checkPostDay("2026-10-10", now), true);
  assert.equal(checkPostDay("2026-10-11", now), true);
  assert.equal(checkPostDay("2026-10-12", now), true);
  assert.equal(checkPostDay("2026-10-09", now), false);
  assert.equal(checkPostDay("2026-10-13", now), false);
});

test("the read window reaches back 90 days", () => {
  const now = Date.UTC(2026, 9, 11, 12);
  assert.equal(checkGetDay("2026-07-13", now), true);
  assert.equal(checkGetDay("2026-07-12", now), false);
});

test("summarize builds the distribution", () => {
  assert.deepEqual(
    summarize(5, [
      { guesses: 0, n: 2 },
      { guesses: 1, n: 1 },
      { guesses: 4, n: 5 },
      { guesses: 9, n: 100 },
    ]),
    { played: 8, solved: 6, distribution: [1, 0, 0, 5, 0, 0] },
  );
});

test("bucket stores a loss as zero", () => {
  assert.equal(bucket({ solved: false, guesses: 6 }), 0);
  assert.equal(bucket({ solved: true, guesses: 4 }), 4);
});
