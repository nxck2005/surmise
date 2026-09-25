// The one-shot permission a clipboard write needs.
//
// Bubble Tea writes copy requests as OSC 52, and xterm.js deliberately leaves
// that to its host, so boot.js bridges it to the browser's Clipboard API. The
// problem is that a bridge registered for the life of the page trusts whoever
// writes to the terminal stream: any OSC 52 reaching it would overwrite the
// player's clipboard, silently, with no signal to them — the game says "copy
// requested" and never learns whether it worked.
//
// So permission is not a property of the page, it is a property of one request.
// internal/web calls armClipboard immediately before it writes a frame carrying
// the introducer, and takeClipboard spends it. One grant, one write.
//
// It is a plain script rather than a module, like boot.js, because index.html
// loads it that way and a module would be deferred past the WebAssembly start.
// The global is one name on globalThis, and the smoke test loads this same file
// the same way — which is the point of it being separate. A gate restated inside
// boot.js could only be tested by restating it again.
(function () {
  "use strict";

  let armed = false;

  globalThis.surmiseClipboard = {
    // A grant is armed, and spent by the first write that claims it. Refusing
    // is silent, because the player's own copy either already failed or never
    // happened, and they are owed no error for a request they did not make.
    arm: function armClipboard() {
      armed = true;
    },

    // take reports whether this write is permitted, and spends the permission
    // either way: an unauthorised write must not leave the grant standing for
    // the next one, or a single arm would authorise however many followed.
    take: function takeClipboard() {
      const allowed = armed;
      armed = false;
      return allowed;
    },
  };
})();
