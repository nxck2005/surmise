// Response helpers for the API. Pure: usable from Node tests, no bindings.
//
// Every response below is JSON with the same three headers; a 204 carries no
// body and keeps the last two, since nothing is being typed.

export const JSON_HEADERS: Record<string, string> = {
  "Content-Type": "application/json; charset=utf-8",
  "Cache-Control": "no-store",
  "X-Content-Type-Options": "nosniff",
};

export function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: JSON_HEADERS });
}

export function noContent(): Response {
  const headers: Record<string, string> = {
    "Cache-Control": "no-store",
    "X-Content-Type-Options": "nosniff",
  };
  return new Response(null, { status: 204, headers });
}

// body: {"error": code} plus "message" when given
export function error(status: number, code: string, message?: string): Response {
  const body: Record<string, string> = { error: code };
  if (message !== undefined) body.message = message;
  return json(status, body);
}

// 410, body {"message": message}
export function gone(message: string): Response {
  return json(410, { message });
}

export async function readJSON(request: Request, maxBytes: number):
  Promise<{ ok: true; value: unknown } | { ok: false; response: Response }> {
  const length = request.headers.get("Content-Length");
  if (length !== null && Number(length) > maxBytes) {
    return { ok: false, response: error(413, "too_large") };
  }
  const text = await request.text();
  if (text.length > maxBytes) {
    return { ok: false, response: error(413, "too_large") };
  }
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { ok: false, response: error(400, "bad_request", "body is not JSON") };
  }
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { ok: false, response: error(400, "bad_request", "body must be an object") };
  }
  return { ok: true, value };
}
