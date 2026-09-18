package handlers

import (
    "net/http"
    "strconv"
)

// parsePagination reads optional "limit" and "offset" query params from the
// request, clamping limit to (0, maxLimit] and defaulting to defaultLimit
// when the param is absent or not a positive integer. offset defaults to 0
// and is clamped to >= 0.
//
// Every list endpoint previously had either a hardcoded LIMIT 50 or no
// limit at all - a caller that doesn't pass either param gets exactly the
// old default page size, so adding this is purely additive; it doesn't
// change any existing response.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
    limit = defaultLimit
    if v := r.URL.Query().Get("limit"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            limit = n
        }
    }
    if limit > maxLimit {
        limit = maxLimit
    }

    if v := r.URL.Query().Get("offset"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n >= 0 {
            offset = n
        }
    }
    return limit, offset
}
