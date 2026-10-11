import { ANSWERS, GUESSES } from "./gen/words.ts";

// isGuess reports whether word is accepted on a board of this length.
export function isGuess(length: number, word: string): boolean {
  return GUESSES[length]?.has(word) === true;
}

// pickAnswer returns a uniformly random entry of ANSWERS[length], using
// crypto.getRandomValues(new Uint32Array(1)) with rejection sampling: draw
// again while the value is >= Math.floor(2**32 / n) * n, so every entry stays
// equally likely even though the list sizes are not powers of two.
export function pickAnswer(length: number): string {
  const answers = ANSWERS[length];
  const n = answers.length;
  const limit = Math.floor(2 ** 32 / n) * n;
  const draw = new Uint32Array(1);
  do {
    crypto.getRandomValues(draw);
  } while (draw[0] >= limit);
  return answers[draw[0] % n];
}
