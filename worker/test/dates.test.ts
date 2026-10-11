import { test } from "node:test";
import assert from "node:assert/strict";
import { dayOffset, parseDay } from "../src/dates.ts";

test("parseDay accepts real dates only", () => {
  assert.equal(parseDay("2026-10-11"), Date.UTC(2026, 9, 11));
  assert.equal(parseDay("2026-02-30"), null);
  assert.equal(parseDay("2026-13-01"), null);
  assert.equal(parseDay("26-10-11"), null);
  assert.equal(parseDay("2026-10-11T00:00"), null);
});

test("dayOffset counts UTC days", () => {
  const now = Date.UTC(2026, 9, 11, 23, 59);
  assert.equal(dayOffset("2026-10-11", now), 0);
  assert.equal(dayOffset("2026-10-10", now), -1);
  assert.equal(dayOffset("2026-10-12", now), 1);
  assert.equal(dayOffset("bad", now), null);
});
