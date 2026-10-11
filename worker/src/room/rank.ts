// The ranking order, Appendix A.7: solved first, then fewer guesses, then
// less time, then the lower seat. Pure.

import type { RankEntry } from "./protocol.ts";

export function rank(entries: readonly RankEntry[]): RankEntry[] {
  return [...entries].sort((a, b) => {
    if (a.solved !== b.solved) return a.solved ? -1 : 1;
    if (a.guesses !== b.guesses) return a.guesses - b.guesses;
    if (a.elapsedMs !== b.elapsedMs) return a.elapsedMs - b.elapsedMs;
    return a.seat - b.seat;
  });
}
