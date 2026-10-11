import { test } from "node:test";
import assert from "node:assert/strict";
import { rank } from "../src/room/rank.ts";
import type { RankEntry } from "../src/room/protocol.ts";

test("rank orders by solved, guesses, time, seat", () => {
  const entries = [
    { seat: 1, name: "", solved: false, guesses: 6, elapsedMs: 600000, rows: [] },
    { seat: 2, name: "", solved: true, guesses: 4, elapsedMs: 90000, rows: [] },
    { seat: 3, name: "", solved: true, guesses: 3, elapsedMs: 200000, rows: [] },
    { seat: 4, name: "", solved: true, guesses: 4, elapsedMs: 80000, rows: [] },
    { seat: 5, name: "", solved: true, guesses: 4, elapsedMs: 80000, rows: [] },
  ] as RankEntry[];
  const ranked = rank(entries);
  assert.deepEqual(ranked.map((e) => e.seat), [3, 4, 5, 2, 1]);
  // The input array is unchanged afterwards.
  assert.deepEqual(entries.map((e) => e.seat), [1, 2, 3, 4, 5]);
});
