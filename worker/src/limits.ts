// Rate limits. Workers-only types: RateLimit is the binding type wrangler
// generates, so this module is type-checked against the runtime but never run
// by the Node tests. No runtime import.
//
// An account where the binding is refused must not take the API down with it:
// a missing limiter means "allowed", and Cloudflare's edge still stands in
// front of floods.

export async function allowed(limiter: RateLimit | undefined, request: Request): Promise<boolean> {
  if (limiter === undefined) return true;
  const key = request.headers.get("CF-Connecting-IP") ?? "unknown";
  const result = await limiter.limit({ key });
  return result.success;
}
