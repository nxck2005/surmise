// Route matching. Pure: match on the pathname only, with query strings and the
// method left to the caller. A trailing slash is not accepted —
// /api/v1/status/ is not found.

export type Route =
  | { name: "status" }
  | { name: "daily"; day: string; length: string }
  | { name: "rooms" } // POST /api/v1/rooms
  | { name: "room_socket"; code: string } // GET  /api/v1/rooms/<code>/socket
  | { name: "room_results"; code: string } // GET  /api/v1/rooms/<code>/results
  | { name: "not_found" };

export interface Match { route: Route; methods: readonly string[] }

const DAILY_PREFIX = "/api/v1/daily/";
const ROOMS_PREFIX = "/api/v1/rooms";

export function match(pathname: string): Match {
  if (pathname === "/api/v1/status") {
    return { route: { name: "status" }, methods: ["GET"] };
  }
  if (pathname.startsWith(ROOMS_PREFIX)) {
    if (pathname === ROOMS_PREFIX) {
      return { route: { name: "rooms" }, methods: ["POST"] };
    }
    // <code> is one non-empty segment; the route handlers check its shape and
    // answer 404, so any segment reaches them.
    const rest = pathname.slice(ROOMS_PREFIX.length + 1);
    const sep = rest.indexOf("/");
    if (sep > 0) {
      const tail = rest.slice(sep + 1);
      if (tail === "socket") {
        return { route: { name: "room_socket", code: rest.slice(0, sep) }, methods: ["GET"] };
      }
      if (tail === "results") {
        return { route: { name: "room_results", code: rest.slice(0, sep) }, methods: ["GET"] };
      }
    }
  }
  if (pathname.startsWith(DAILY_PREFIX)) {
    const rest = pathname.slice(DAILY_PREFIX.length);
    const sep = rest.indexOf("/");
    const tail = rest.slice(sep + 1);
    // <day> and <length> are each one non-empty path segment without /.
    if (sep > 0 && tail !== "" && !tail.includes("/")) {
      return {
        route: { name: "daily", day: rest.slice(0, sep), length: tail },
        methods: ["GET", "POST"],
      };
    }
  }
  return { route: { name: "not_found" }, methods: [] };
}
