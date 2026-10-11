// The client header: every request names the client that sent it, so the
// server can turn a version away. Pure.

export interface ClientInfo { name: string; version: string; platform: string }

export const CLIENT_HEADER = "X-Surmise-Client";

// "surmise/0.9.0 (linux/amd64)" — name, version, then the platform in
// parentheses. The version accepts the shapes a Go build stamps: a bare
// semver, a leading v, and a pseudo-version with a date suffix.
const CLIENT_PATTERN =
  /^([a-z0-9-]{1,32})\/([0-9A-Za-z.+-]{1,64}) \(([a-z0-9]{1,16}\/[a-z0-9]{1,16})\)$/;

export function parseClient(value: string | null): ClientInfo | null {
  if (value === null) return null;
  const m = CLIENT_PATTERN.exec(value);
  if (m === null) return null;
  return { name: m[1], version: m[2], platform: m[3] };
}

const VERSION_PATTERN = /^v?(\d+)\.(\d+)\.(\d+)/;

export function isTooOld(version: string, min: string): boolean {
  if (min === "") return false;
  const have = VERSION_PATTERN.exec(version);
  if (have === null) return false; // "dev": development builds are never turned away
  const want = VERSION_PATTERN.exec(min);
  if (want === null) return false;
  for (let i = 1; i <= 3; i++) {
    const part = Number(have[i]);
    const floor = Number(want[i]);
    if (part !== floor) return part < floor;
  }
  return false;
}
