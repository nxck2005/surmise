import { test } from "node:test";
import assert from "node:assert/strict";
import {
  CODE_PATTERN, KEY_PATTERN, freshKey, isCode, isKey, randomCode, randomKey, sha256hex,
} from "../src/codes.ts";

test("randomCode uses the alphabet", () => {
  for (let i = 0; i < 200; i++) {
    assert.match(randomCode(), CODE_PATTERN);
  }
});

test("isCode accepts codes only", () => {
  assert.equal(isCode("K7M2QX"), true);
  assert.equal(isCode("k7m2qx"), false); // codes are capitals
  assert.equal(isCode("K7M2Q"), false);
  assert.equal(isCode("IO0OIL"), false); // I, L, O, U are not in the alphabet
});

test("randomKey is 32 hex", () => {
  const key = randomKey();
  assert.match(key, KEY_PATTERN);
  assert.equal(isKey(key), true);
  assert.equal(isKey("abc"), false);
});

test("freshKey pairs a key with its hash", async () => {
  const { key, keyHash } = await freshKey();
  assert.match(key, KEY_PATTERN);
  assert.equal(keyHash, await sha256hex(key));
});

test("sha256hex is the known digest", async () => {
  assert.equal(await sha256hex("abc"),
    "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
});
