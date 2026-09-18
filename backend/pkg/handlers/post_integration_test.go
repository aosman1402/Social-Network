package handlers

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "testing"

    "social-network3/pkg/middleware"
)

// withUserID attaches userID to the request context the same way
// middleware.AuthMiddleware.Authenticate does, so a handler can be called
// directly in a test without going through a real session cookie.
func withUserID(r *http.Request, userID int64) *http.Request {
    ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
    return r.WithContext(ctx)
}

// registerTestUser creates a user directly via the Register handler and
// returns their id - reusing the real handler (rather than inserting a row
// by hand) so the fixture stays in sync with whatever Register actually
// requires.
func registerTestUser(t *testing.T, db *sql.DB, email string) int64 {
    t.Helper()
    auth := &AuthHandler{DB: db}
    w := doJSON(t, auth.Register, http.MethodPost, "/api/register", map[string]interface{}{
        "email":         email,
        "password":      "Sup3rSec9!zx",
        "first_name":    "Test",
        "last_name":     "User",
        "date_of_birth": "1990-01-01",
    }, nil)
    if w.Code != http.StatusCreated {
        t.Fatalf("registerTestUser: status = %d, body = %s", w.Code, w.Body.String())
    }
    var resp struct {
        UserID int64 `json:"user_id"`
    }
    if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
        t.Fatalf("registerTestUser: failed to parse response: %v", err)
    }
    return resp.UserID
}

// newMultipartPostRequest builds a CreatePost request the way the frontend
// does - multipart/form-data, not JSON (needed for optional image uploads).
func newMultipartPostRequest(t *testing.T, fields map[string]string) *http.Request {
    t.Helper()
    var buf bytes.Buffer
    mw := multipart.NewWriter(&buf)
    for k, v := range fields {
        if err := mw.WriteField(k, v); err != nil {
            t.Fatalf("failed to write multipart field %q: %v", k, err)
        }
    }
    if err := mw.Close(); err != nil {
        t.Fatalf("failed to close multipart writer: %v", err)
    }
    r := httptest.NewRequest(http.MethodPost, "/api/post/create", &buf)
    r.Header.Set("Content-Type", mw.FormDataContentType())
    return r
}

func TestCreateAndListPosts(t *testing.T) {
    db := newTestDB(t)
    post := &PostHandler{DB: db}
    userID := registerTestUser(t, db, "poster@example.com")

    r := withUserID(newMultipartPostRequest(t, map[string]string{
        "content":       "hello, world",
        "privacy_level": "public",
    }), userID)
    w := httptest.NewRecorder()
    post.CreatePost(w, r)
    if w.Code != http.StatusCreated {
        t.Fatalf("CreatePost: status = %d, body = %s", w.Code, w.Body.String())
    }

    // GetPosts must return the post with the full shape scanPostRow
    // produces - author, counts, and this viewer's like/save state -
    // consistently, the same shape regardless of which list endpoint
    // (feed, profile, or group) is asked.
    listReq := withUserID(httptest.NewRequest(http.MethodGet, "/api/posts", nil), userID)
    listW := httptest.NewRecorder()
    post.GetPosts(listW, listReq)
    if listW.Code != http.StatusOK {
        t.Fatalf("GetPosts: status = %d, body = %s", listW.Code, listW.Body.String())
    }

    var posts []map[string]interface{}
    if err := json.Unmarshal(listW.Body.Bytes(), &posts); err != nil {
        t.Fatalf("GetPosts: failed to parse response JSON: %v (body: %s)", err, listW.Body.String())
    }
    if len(posts) != 1 {
        t.Fatalf("GetPosts: got %d posts, want 1", len(posts))
    }

    p := posts[0]
    if p["content"] != "hello, world" {
        t.Errorf("post content = %v, want %q", p["content"], "hello, world")
    }
    for _, field := range []string{"like_count", "is_liked", "is_saved", "comment_count", "user"} {
        if _, ok := p[field]; !ok {
            t.Errorf("post response missing field %q: %v", field, p)
        }
    }
}
