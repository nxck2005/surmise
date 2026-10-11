import { test } from "node:test";
import assert from "node:assert/strict";
import { cleanName } from "../src/names.ts";

test("cleanName follows the rules", () => {
  assert.equal(cleanName("Nishchal"), "Nishchal");
  assert.equal(cleanName("  sam  "), "sam");
  assert.equal(cleanName("a very long name here"), "a very long");
  assert.equal(cleanName("émile"), "mile");
  assert.equal(cleanName("<b>x</b>"), "bxb");
  assert.equal(cleanName("abbo123"), "");
  assert.equal(cleanName("xabbo"), "xabbo");
  assert.equal(cleanName("!!!"), "");
});
