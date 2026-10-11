import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { score } from "../src/score.ts";

interface Vector {
  guess: string;
  answer: string;
  marks: number[];
}

test("score matches game.Score on every vector", () => {
  const text = readFileSync(new URL("./score-vectors.json", import.meta.url), "utf8");
  const vectors = JSON.parse(text) as Vector[];
  assert.ok(vectors.length > 0);
  for (const v of vectors) {
    assert.deepEqual(score(v.guess, v.answer), v.marks, `${v.guess} against ${v.answer}`);
  }
});

test("score throws on mismatched lengths", () => {
  assert.throws(() => score("crane", "abcd"), /mismatched lengths/);
});
