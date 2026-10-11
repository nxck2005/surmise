// Validation for the daily routes. Pure.
import { dayOffset, utcDay } from "./dates.ts";

export const LENGTHS: readonly number[] = [4, 5, 6];

export interface DailyPost { solved: boolean; guesses: number }

export function parseLength(s: string): number | null {
  for (const n of LENGTHS) {
    if (s === String(n)) return n;
  }
  return null;
}

export function checkPostDay(day: string, nowMs: number): boolean {
  const offset = dayOffset(day, nowMs);
  return offset !== null && offset >= -1 && offset <= 1;
}

export function checkGetDay(day: string, nowMs: number): boolean {
  const offset = dayOffset(day, nowMs);
  return offset !== null && offset >= -90 && offset <= 1;
}

// string = the error message for a 400.
export function parsePost(body: unknown, length: number): DailyPost | string {
  if (typeof body !== "object" || body === null || Array.isArray(body)) {
    return "solved must be true or false";
  }
  const b = body as Record<string, unknown>;
  if (typeof b.solved !== "boolean") return "solved must be true or false";
  if (!Number.isInteger(b.guesses)) return "guesses must be a whole number";
  const solved = b.solved as boolean;
  const guesses = b.guesses as number;
  if (solved && (guesses < 1 || guesses > length + 1)) return "guesses is out of range";
  if (!solved && guesses !== length + 1) return "guesses is out of range";
  return { solved, guesses };
}

export function bucket(p: DailyPost): number {
  return p.solved ? p.guesses : 0;
}

export function summarize(length: number, rows: { guesses: number; n: number }[]):
  { played: number; solved: number; distribution: number[] } {
  const distribution = new Array<number>(length + 1).fill(0);
  let played = 0;
  let solved = 0;
  for (const row of rows) {
    if (row.guesses < 0 || row.guesses > length + 1) continue; // ignored entirely
    played += row.n; // the loss bucket is somebody who played and did not solve
    if (row.guesses >= 1) {
      solved += row.n;
      distribution[row.guesses - 1] += row.n;
    }
  }
  return { played, solved, distribution };
}

// utcDay(nowMs - 90 days)
export function cutoffDay(nowMs: number): string {
  return utcDay(nowMs - 90 * 24 * 60 * 60 * 1000);
}
