package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestParsePagination(t *testing.T) {
    cases := []struct {
        name           string
        query          string
        defaultLimit   int
        maxLimit       int
        wantLimit      int
        wantOffset     int
    }{
        {"no params uses default", "", 50, 100, 50, 0},
        {"explicit limit and offset", "?limit=10&offset=20", 50, 100, 10, 20},
        {"limit over max is clamped", "?limit=1000", 50, 100, 100, 0},
        {"zero limit falls back to default", "?limit=0", 50, 100, 50, 0},
        {"negative limit falls back to default", "?limit=-5", 50, 100, 50, 0},
        {"non-numeric limit falls back to default", "?limit=abc", 50, 100, 50, 0},
        {"negative offset falls back to zero", "?offset=-5", 50, 100, 50, 0},
        {"non-numeric offset falls back to zero", "?offset=abc", 50, 100, 50, 0},
    }

    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            r := httptest.NewRequest(http.MethodGet, "/x"+c.query, nil)
            gotLimit, gotOffset := parsePagination(r, c.defaultLimit, c.maxLimit)
            if gotLimit != c.wantLimit || gotOffset != c.wantOffset {
                t.Errorf("parsePagination(%q) = (%d, %d), want (%d, %d)",
                    c.query, gotLimit, gotOffset, c.wantLimit, c.wantOffset)
            }
        })
    }
}
