// Thin wrapper around fetch() for talking to the backend API. Centralizes
// the base URL, the session cookie (credentials: "include"), and JSON
// body/response handling, which every call site was otherwise repeating.
//
// Deliberately mirrors the raw fetch()+response.json() pattern already used
// everywhere in this codebase - it returns { ok, status, data } rather than
// throwing on a non-2xx response - so callers keep their existing
// `if (!ok) ...` shape instead of being rewritten around try/catch.
export const API_BASE = "http://localhost:8081/api";

/**
 * @param {string} path - API path, e.g. "/login" (leading slash required).
 * @param {object} [options]
 * @param {any} [options.json] - If given, sent as the JSON-encoded body with
 *   Content-Type: application/json set automatically. Omit this and pass a
 *   FormData instance as `body` directly for file uploads.
 * @param {...*} [options.rest] - Any other fetch() option (method, body,
 *   headers, etc.), passed through as-is.
 */
export async function apiFetch(path, { json, headers, ...rest } = {}) {
  const finalHeaders = { ...headers };
  let body = rest.body;
  if (json !== undefined) {
    finalHeaders["Content-Type"] = "application/json";
    body = JSON.stringify(json);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...rest,
    headers: finalHeaders,
    body,
  });

  let data = null;
  try {
    data = await response.json();
  } catch {
    // Some endpoints (e.g. logout) return no body.
  }

  return { ok: response.ok, status: response.status, data };
}
