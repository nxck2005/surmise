// Room codes and seat keys. Pure: the patterns are wire format, the random
// bytes come from the platform's crypto, and Node's tests run the same code.
//
// A code is six characters of a Crockford-like alphabet — no I, L, O or U, so
// a code read aloud or written down survives its lowercase.
export const CODE_ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";
export const CODE_PATTERN = /^[0-9A-HJKMNP-TV-Z]{6}$/;
export const KEY_PATTERN = /^[0-9a-f]{32}$/;

export function isCode(s: string): boolean {
  return CODE_PATTERN.test(s);
}

export function isKey(s: string): boolean {
  return KEY_PATTERN.test(s);
}

export function randomCode(): string {
  // 32 divides 256, so masking is uniform.
  const bytes = crypto.getRandomValues(new Uint8Array(6));
  let code = "";
  for (const b of bytes) code += CODE_ALPHABET[b & 31];
  return code;
}

export function randomKey(): string {
  return [...crypto.getRandomValues(new Uint8Array(16))]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export async function sha256hex(s: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s));
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// freshKey makes a new seat key and the hash of it the room stores. The key
// itself is only ever handed to the player; the room keeps the hash.
export async function freshKey(): Promise<{ key: string; keyHash: string }> {
  const key = randomKey();
  return { key, keyHash: await sha256hex(key) };
}
