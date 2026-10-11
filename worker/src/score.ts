// score compares a guess with the answer: one mark per letter, 2 correct,
// 1 present, 0 absent. It is a line-for-line port of game.Score in
// internal/game/score.go, and test/score-vectors.json (written by
// tools/genworker from the Go function) holds the two to the same answers.
// Duplicate letters: exact matches are claimed first, then leftover letters
// are handed out left to right.
export function score(guess: string, answer: string): number[] {
  if (guess.length !== answer.length) {
    throw new Error("score: mismatched lengths");
  }

  const marks = new Array<number>(guess.length).fill(0);
  const remaining = new Array<number>(26).fill(0);

  for (let i = 0; i < answer.length; i++) {
    const idx = answer.charCodeAt(i) - 97;
    if (idx >= 0 && idx < 26) remaining[idx]++;
  }

  for (let i = 0; i < guess.length; i++) {
    if (guess.charCodeAt(i) === answer.charCodeAt(i)) {
      marks[i] = 2;
      const idx = guess.charCodeAt(i) - 97;
      if (idx >= 0 && idx < 26) remaining[idx]--;
    }
  }

  for (let i = 0; i < guess.length; i++) {
    if (marks[i] === 2) continue;
    const idx = guess.charCodeAt(i) - 97;
    if (idx >= 0 && idx < 26 && remaining[idx] > 0) {
      marks[i] = 1;
      remaining[idx]--;
    }
  }

  return marks;
}
