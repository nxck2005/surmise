import { test } from "node:test";
import assert from "node:assert/strict";
import { ANSWERS, GUESSES } from "../src/gen/words.ts";
import { isGuess, pickAnswer } from "../src/words.ts";

test("every answer is a guess", () => {
  for (const n of [4, 5, 6]) {
    for (const answer of ANSWERS[n]) {
      assert.match(answer, /^[a-z]+$/, answer);
      assert.equal(answer.length, n, answer);
      assert.ok(GUESSES[n].has(answer), `${n}: ${answer}`);
      assert.ok(isGuess(n, answer), `${n}: ${answer}`);
    }
  }
});

test("pickAnswer returns an answer", () => {
  for (let i = 0; i < 50; i++) {
    assert.ok(ANSWERS[5].includes(pickAnswer(5)));
  }
});
