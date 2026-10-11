// UTC day arithmetic. Pure: Node dates are UTC-capable, the same as Workers.

const DAY_PATTERN = /^\d{4}-\d{2}-\d{2}$/;
const MS_PER_DAY = 24 * 60 * 60 * 1000;

// "YYYY-MM-DD" → milliseconds at 00:00 UTC that day, or null. Must match
// /^\d{4}-\d{2}-\d{2}$/ and round-trip through Date, so 2026-02-30 and
// 2026-10-11T00:00 are refused.
export function parseDay(s: string): number | null {
  if (!DAY_PATTERN.test(s)) return null;
  const ms = Date.UTC(Number(s.slice(0, 4)), Number(s.slice(5, 7)) - 1, Number(s.slice(8, 10)));
  if (new Date(ms).toISOString().slice(0, 10) !== s) return null;
  return ms;
}

// ms → "YYYY-MM-DD" (UTC)
export function utcDay(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10);
}

// whole days from today (UTC, from nowMs) to day: 0 today, -1 yesterday,
// +1 tomorrow; null when day does not parse.
export function dayOffset(day: string, nowMs: number): number | null {
  const want = parseDay(day);
  if (want === null) return null;
  const today = parseDay(utcDay(nowMs));
  if (today === null) return null;
  return (want - today) / MS_PER_DAY;
}
