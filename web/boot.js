// The page half of the browser build.
//
// The contract with the Go half is one object: globalThis.surmise = { term,
// onExit }. internal/web reads it, wires xterm.js up as bubbletea's input and
// output, and never touches the DOM beyond that.

const $ = (id) => document.getElementById(id);

const term = new Terminal({
  // The game draws its own cursor as a highlighted tile.
  cursorBlink: false,
  cursorStyle: "bar",
  cursorInactiveStyle: "none",
  // Nothing here scrolls back: the app owns the alternate screen.
  scrollback: 0,
  fontFamily:
    'ui-monospace, "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace',
  fontSize: 15,
  // Read from the stylesheet so the page and the terminal start on one colour.
  theme: readTheme(),
  allowProposedApi: true,
});

const fit = new FitAddon.FitAddon();
term.loadAddon(fit);
term.open($("terminal"));
fit.fit();

function readTheme() {
  const style = getComputedStyle(document.documentElement);
  return {
    background: style.getPropertyValue("--surmise-bg").trim(),
    foreground: style.getPropertyValue("--surmise-fg").trim(),
  };
}

// Bubble Tea sets the palette with OSC 10 (foreground) and OSC 11 (background).
// Mirroring those onto the page is what makes the theme picker recolour the
// letterboxing around the board, instead of leaving a fixed frame around a
// changing game. Returning false lets xterm.js apply them to itself as usual.
const paint = (name) => (data) => {
  if (/^#[0-9a-f]{6}$/i.test(data)) {
    document.documentElement.style.setProperty(name, data);
  }
  return false;
};
term.parser.registerOscHandler(10, paint("--surmise-fg"));
term.parser.registerOscHandler(11, paint("--surmise-bg"));

// Bubble Tea writes clipboard requests as OSC 52. xterm.js deliberately leaves
// that sequence to its host, so bridge system-clipboard writes to the browser.
// The game emits this only for an explicit result-screen copy action.
term.parser.registerOscHandler(52, async (data) => {
  const match = /^c;([A-Za-z0-9+/]*={0,2})$/.exec(data);
  if (!match) return false;

  let text;
  try {
    const bytes = Uint8Array.from(atob(match[1]), (c) => c.charCodeAt(0));
    text = new TextDecoder().decode(bytes);
  } catch (error) {
    console.warn("ignored malformed clipboard payload", error);
    return true;
  }

  if (!navigator.clipboard?.writeText) {
    console.warn("browser clipboard is unavailable");
    return true;
  }
  try {
    await navigator.clipboard.writeText(text);
  } catch (error) {
    console.warn("could not write browser clipboard", error);
  }
  return true;
});

// Let the browser keep the chords it needs: copy when there is a selection, and
// paste. Everything else belongs to the game.
term.attachCustomKeyEventHandler((e) => {
  if (e.type !== "keydown") return true;
  const mod = e.ctrlKey || e.metaKey;
  if (mod && e.key === "c" && term.hasSelection()) return false;
  if (mod && e.key === "v") return false;
  return true;
});

// Refit on any container change, not just a window resize — the address bar
// appearing on a phone is a resize the window event does not report.
let pending = 0;
new ResizeObserver(() => {
  clearTimeout(pending);
  pending = setTimeout(() => fit.fit(), 50);
}).observe($("terminal"));

// Clicking the frame focuses the game, so a stray click does not leave the
// keyboard pointing at the page.
$("terminal").addEventListener("mousedown", () => term.focus());

globalThis.surmise = {
  term,
  onExit() {
    $("exit").hidden = false;
  },
};

$("again").addEventListener("click", () => location.reload());

function fail(message) {
  $("loading").hidden = true;
  $("error-text").textContent = message;
  $("error").hidden = false;
}

async function main() {
  if (!globalThis.WebAssembly) {
    fail("This browser has no WebAssembly.");
    return;
  }
  const go = new Go();
  try {
    // instantiateStreaming compiles while the module downloads, which is worth
    // having: the binary is a few megabytes.
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch(wasmURL()),
      go.importObject,
    );
    $("loading").hidden = true;
    term.focus();
    await go.run(instance);
  } catch (err) {
    fail(String(err));
  }
}

// The build script rewrites this with a cache-busting query.
function wasmURL() {
  return "surmise.wasm";
}

main();
