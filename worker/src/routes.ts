// Route matching. Pure: match on the pathname only, with query strings and the
// method left to the caller. A trailing slash is not accepted —
// /api/v1/status/ is not found.

export type Route =
  | { name: "status" }
  | { name: "daily"; day: string; length: string }
  | { name: "not_found" };

export interface Match { route: Route; methods: readonly string[] }

const DAILY_PREFIX = "/api/v1/daily/";

export function match(pathname: string): Match {
  if (pathname === "/api/v1/status") {
    return { route: { name: "status" }, methods: ["GET"] };
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
