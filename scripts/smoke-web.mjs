// Run the WebAssembly build headlessly, against a stub xterm.js.
//
//   node scripts/smoke-web.mjs web/dist/surmise.wasm
//
// This is not a substitute for opening the page — colour, layout and hover
// still need eyes. What it does cover is everything between the Go program and
// the terminal: that the runtime comes up, that internal/web finds the page
// object, that keystrokes pushed through a callback reach the board, that a
// resize sent from a callback is delivered rather than deadlocking, and that
// the colour profile is forced.
//
// The last two are the failures worth catching automatically. A blocking
// js.Func callback hangs the runtime, and a missed WithColorProfile renders the
// whole game in monochrome — neither shows up in a compile.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const goroot = execSync("go env GOROOT").toString().trim();

// wasm_exec.js expects a browser-ish global scope.
globalThis.require = require;
globalThis.fs = require("node:fs");
globalThis.path = require("node:path");
globalThis.TextEncoder = TextEncoder;
globalThis.TextDecoder = TextDecoder;
globalThis.performance = performance;

require(`${goroot}/lib/wasm/wasm_exec.js`);

// The stub terminal. It records the callbacks the Go side registers and keeps
// everything written to it, which is the whole of the contract in term_js.go.
const handlers = {};
const dec = new TextDecoder();
let out = "";

const term = {
  cols: 80,
  rows: 30,
  write(b) {
    out += typeof b === "string" ? b : dec.decode(b);
  },
  onData: (f) => (handlers.data = f),
  onBinary: (f) => (handlers.binary = f),
  onResize: (f) => (handlers.resize = f),
  onTitleChange: (f) => (handlers.title = f),
};

globalThis.document = { title: "" };
let exited = false;
globalThis.surmise = { term, onExit: () => (exited = true) };

const go = new Go();
const wasmPath = process.argv[2] ?? "web/dist/surmise.wasm";
const { instance } = await WebAssembly.instantiate(
  readFileSync(wasmPath),
  go.importObject,
);
go.run(instance);

const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const type = async (s) => {
  handlers.data?.(s);
  await wait(120);
};

await wait(600); // the splash, and the first frame

const results = [];
const check = (label, ok) => results.push({ label, ok });

check("wrote a frame", out.length > 0);
check("used the alternate screen", out.includes("\x1b[?1049h"));
check("enabled mouse tracking", /\x1b\[\?100[023]h/.test(out));
check("set the window title", /\x1b]2;/.test(out));
check("set the background colour", /\x1b]11;/.test(out));
// The one that fails silently in a browser: no profile means no colour.
check("emitted 24-bit colour", /\x1b\[[34]8;2;/.test(out));

await type("\r"); // dismiss the splash

out = "";
for (const ch of "crane") await type(ch);
check("typed letters reach the board", /[CRANE]/.test(out));

// A resize arrives on a callback and is delivered by a goroutine. If that hand
// -off ever blocks, this is where the runtime hangs.
out = "";
handlers.resize?.({ cols: 100, rows: 40 });
await wait(250);
check("a resize redraws", out.length > 0);

out = "";
await type("\x1b");
check("esc opens the menu", out.length > 0);

for (const { label, ok } of results) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}`);
}
const failed = results.filter((r) => !r.ok);
console.log(failed.length ? `\n${failed.length} failed` : "\nall passed");
process.exit(failed.length ? 1 : 0);
