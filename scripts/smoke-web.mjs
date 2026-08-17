// Run the WebAssembly build headlessly against the real xterm.js engine.
//
//   node scripts/smoke-web.mjs web/dist/surmise.wasm
//
// @xterm/headless is the terminal the page uses, without a DOM, so the screen
// buffer here is authoritative: what this inspects is what a browser shows.
//
// It is not a substitute for opening the page — fonts, colour fidelity and
// hover latency still need eyes. What it covers is everything between the Go
// program, terminal and browser storage, including four failures a compile
// cannot see: a blocked js.Func hangs the runtime, a missing colour profile
// renders in monochrome, non-tty newline mapping leaves stale cells, and a
// localStorage failure silently forgets history on reload.
import { execSync, spawnSync } from "node:child_process";
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const phase = process.argv[3];
const wasm = resolve(process.argv[2] ?? "web/dist/surmise.wasm");

// Persistence needs two browser lifetimes. Run this file once to save a puzzle
// and again against the same Web Storage file to prove the next instance reads
// it back. Keeping the orchestration here preserves the one-command smoke test.
if (!phase) {
  const dir = mkdtempSync(join(tmpdir(), "surmise-web-smoke-"));
  const storage = join(dir, "localstorage.json");
  const script = fileURLToPath(import.meta.url);
  let status = 0;

  try {
    for (const childPhase of ["write", "read"]) {
      const child = spawnSync(
        process.execPath,
        [script, wasm, childPhase, storage],
        { stdio: "inherit" },
      );
      if (child.status !== 0) {
        status = child.status ?? 1;
        break;
      }
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  process.exit(status);
}

// Node does not expose Web Storage consistently across supported versions.
// This file-backed implementation has the browser API used by LocalStorage and
// lets the two child processes model a reload without another dependency.
class FileStorage {
  constructor(filename) {
    this.filename = filename;
    this.values = existsSync(filename)
      ? JSON.parse(readFileSync(filename, "utf8"))
      : {};
  }

  get length() {
    return Object.keys(this.values).length;
  }

  key(index) {
    return Object.keys(this.values)[index] ?? null;
  }

  getItem(key) {
    return Object.prototype.hasOwnProperty.call(this.values, key)
      ? this.values[key]
      : null;
  }

  setItem(key, value) {
    this.values[key] = String(value);
    this.save();
  }

  removeItem(key) {
    delete this.values[key];
    this.save();
  }

  save() {
    writeFileSync(this.filename, JSON.stringify(this.values));
  }
}

globalThis.localStorage = new FileStorage(process.argv[4]);
const require = createRequire(import.meta.url);

// The engine lives with the page's other pinned JS, in web/node_modules.
const { Terminal } = require("../web/node_modules/@xterm/headless");
const goroot = execSync("go env GOROOT").toString().trim();

// wasm_exec.js expects a browser-ish global scope.
globalThis.require = require;
globalThis.fs = require("node:fs");
globalThis.path = require("node:path");
globalThis.TextEncoder = TextEncoder;
globalThis.TextDecoder = TextDecoder;
globalThis.performance = performance;

require(`${goroot}/lib/wasm/wasm_exec.js`);

const term = new Terminal({ cols: 120, rows: 30, allowProposedApi: true });

// The page mirrors these onto the document and the CSS; here they are just
// recorded, so the assertions below can check the program asked for them.
const osc = {};
term.parser.registerOscHandler(10, (d) => ((osc.fg = d), false));
term.parser.registerOscHandler(11, (d) => ((osc.bg = d), false));
let title = "";
term.onTitleChange((t) => (title = t));

let exited = false;
globalThis.document = { title: "" };

// The page's file half, stubbed. It is the same contract boot.js publishes —
// saveFile returns the name it used, openFile answers a callback exactly once —
// so the Go side runs its real path: a js.Func callback dropping an answer into
// a channel that a bubbletea command is blocked on. That hand-off is the one
// this file exists to prove cannot deadlock (see the resize check at the foot).
const files = { saved: null, offer: null };
globalThis.surmise = {
  term,
  onExit: () => (exited = true),
  saveFile(text) {
    files.saved = text;
    return "surmise-backup-smoke.json";
  },
  openFile(done) {
    // Answered on a later turn of the event loop, as a real picker would be:
    // answering inline would prove nothing about the wait.
    setTimeout(() => {
      done(files.offer ? { name: "surmise-backup-smoke.json", text: files.offer } : {});
    }, 20);
  },
};

const go = new Go();
const { instance } = await WebAssembly.instantiate(
  readFileSync(wasm),
  go.importObject,
);
go.run(instance);

const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const type = async (s) => {
  term.input(s);
  await wait(160);
};

const screen = () => {
  const b = term.buffer.active;
  const out = [];
  for (let i = 0; i < term.rows; i++) {
    out.push((b.getLine(b.viewportY + i)?.translateToString(true) ?? "").trimEnd());
  }
  return out;
};

// Every non-blank row of a centred panel starts in the same column. More than
// one left edge means something from an earlier, larger frame is still there.
const leftEdges = (rows) =>
  new Set(rows.filter((l) => l.trim().length > 0).map((l) => l.search(/\S/)));

// True when any cell on screen carries a 24-bit colour, which is what the
// forced colour profile produces and what its absence would remove.
const hasTrueColour = () => {
  const b = term.buffer.active;
  for (let y = 0; y < term.rows; y++) {
    const line = b.getLine(b.viewportY + y);
    if (!line) continue;
    for (let x = 0; x < term.cols; x++) {
      const cell = line.getCell(x);
      if (cell && (cell.isFgRGB() || cell.isBgRGB())) return true;
    }
  }
  return false;
};

const results = [];
const check = (label, ok, detail = "") => results.push({ label, ok, detail });
const report = () => {
  for (const { label, ok, detail } of results) {
    console.log(`${ok ? "PASS" : "FAIL"}  ${label}${!ok && detail ? ` — ${detail}` : ""}`);
  }
  const failed = results.filter((r) => !r.ok);
  if (failed.length) {
    console.log("\n--- screen ---");
    screen().forEach((l, i) => console.log(String(i).padStart(2) + "|" + l));
  }
  console.log(
    failed.length ? `\n${failed.length} failed` : `\nall passed (exit overlay wired: ${exited === false})`,
  );
  process.exit(failed.length ? 1 : 0);
};

await wait(800); // the splash, then the first frame
if (phase === "read") {
  await type("\r"); // dismiss the splash
  await wait(300);
  await type("\x1b"); // open the menu
  await wait(400);
  // Walk down to "puzzles" rather than counting keystrokes to it: the menu has
  // grown before, and a hard-coded count silently opens whatever moved into
  // that slot instead of failing on the row it meant.
  const onPuzzles = () => screen().some((l) => /›\s*puzzles\s*‹/.test(l));
  for (let i = 0; i < 12 && !onPuzzles(); i++) {
    await type("j");
    await wait(60);
  }
  check("found the puzzles row on the menu", onPuzzles());
  await type("\r"); // open puzzles
  await wait(400);

  const puzzles = screen();
  check(
    "loaded the saved puzzle in a fresh browser run",
    puzzles.some((l) => /#\d{6} 5 letters/.test(l)),
    "saved puzzle is absent from the puzzle list",
  );
  report();
}

check("drew a first frame", screen().some((l) => l.length > 0));
check("set the window title", title !== "", `title is ${JSON.stringify(title)}`);
check("asked for a background colour", Boolean(osc.bg), "no OSC 11");
check("rendered in 24-bit colour", hasTrueColour(), "no RGB cell on screen");

await type("\r"); // dismiss the splash
await wait(300);

for (const c of "crane") await type(c);
await type("\r");
await wait(300);

const board = screen();
check("typed letters reach the board", board.some((l) => /C\s+R\s+A\s+N\s+E/.test(l)));
check("the board draws its keyboard", board.some((l) => /Q\s+W\s+E\s+R\s+T/.test(l)));
const puzzleKeys = Array.from(
  { length: localStorage.length },
  (_, i) => localStorage.key(i),
).filter((key) => key?.startsWith("surmise/v1/puzzle/"));
check(
  "saved the puzzle to browser storage",
  puzzleKeys.length === 1,
  `found keys ${JSON.stringify(puzzleKeys)}`,
);
const saved = puzzleKeys.length === 1
  ? JSON.parse(localStorage.getItem(puzzleKeys[0]))
  : null;
check(
  "stored the submitted guess",
  saved?.guesses?.[0] === "crane",
  `first guess is ${JSON.stringify(saved?.guesses?.[0])}`,
);

// The regression this file exists for: the frame shrinks going back to the
// menu. See third_party/bubbletea/tty_js.go.
await type("\x1b");
await wait(400);

const menu = screen();
check("esc reaches the menu", menu.some((l) => l.includes("enter select")));
const edges = leftEdges(menu);
check(
  "the shrinking frame left no stale cells",
  edges.size === 1,
  `content starts at columns ${[...edges].sort((a, b) => a - b).join(", ")}`,
);

// Backing up, which in a browser is the only way a history ever leaves this
// origin. Both directions go through a js.Func callback, so a mistake here does
// not fail — it hangs the runtime, and every check after this one times out.
const onRow = (name) => screen().some((l) => new RegExp(`›\\s*${name}\\s*‹`).test(l));
for (let i = 0; i < 14 && !onRow("backup"); i++) {
  await type("j");
  await wait(60);
}
check("found the backup row on the menu", onRow("backup"));
await type("\r");
await wait(300);

await type("\r"); // the cursor opens on "save a backup"
await wait(400);
let archive = null;
try {
  archive = files.saved ? JSON.parse(files.saved) : null;
} catch (err) {
  archive = null;
}
check(
  "saving offered the page a backup file",
  archive?.format === "surmise.backup" && archive?.puzzles?.length === 1,
  `page received ${JSON.stringify(files.saved)?.slice(0, 120)}`,
);
check(
  "the screen says where the backup went",
  screen().some((l) => l.includes("saved to")),
);

// And back in. The same file, so nothing is new — which is the promise the
// screen makes before the button is pressed.
files.offer = files.saved;
await type("j");
await type("\r");
await wait(600);
check(
  "loading a backup reaches the page and comes back",
  screen().some((l) => l.includes("nothing new")),
  "the load did not report",
);

await type("\x1b"); // back to the menu
await wait(300);

// A resize arrives on a callback and is delivered by a goroutine. If that
// hand-off ever blocks, the runtime hangs here instead of redrawing.
term.resize(100, 24);
await wait(500);
const resized = screen();
check("a resize redraws", resized.some((l) => l.trim().length > 0));
check("the resized frame left no stale cells", leftEdges(resized).size === 1);

report();
