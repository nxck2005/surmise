import { OFFENSIVE } from "./gen/words.ts";

// cleanName turns a player's display name into the name a room shows: the
// rules are in the plan's Part 2 and mirror online.RoomName in Go.
export function cleanName(raw: string): string {
  // Keep only the plain characters a name may carry.
  let name = raw.replace(/[^A-Za-z0-9 ._\-]/g, "");
  // Trim, cut to the first 12 characters, then trim the end again.
  name = name.trim().slice(0, 12).trimEnd();
  // A name whose letter runs are all clean passes; otherwise it is empty.
  for (const piece of name.toLowerCase().split(/[^a-z]+/)) {
    if (OFFENSIVE.has(piece)) return "";
  }
  return name;
}
