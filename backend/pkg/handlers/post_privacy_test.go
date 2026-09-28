package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

// TestCreatePostPrivacyLevels covers the privacy field: the three real
// options are stored as given, an empty one defaults to public, and any
// other value is rejected instead of being stored as a level no visibility
// rule understands.
func TestCreatePostPrivacyLevels(t *testing.T) {
    db := newTestDB(t)
    ph := &PostHandler{DB: db}
    author := registerTestUser(t, db, "privacy-author@example.com")

    cases := []struct {
        level      string
        wantStatus int
        wantStored string
    }{
        {"public", http.StatusCreated, "public"},
        {"almost_private", http.StatusCreated, "almost_private"},
        {"private", http.StatusCreated, "private"},
        {"", http.StatusCreated, "public"},
        {"everyone", http.StatusBadRequest, ""},
    }

    for _, tc := range cases {
        var before int
        db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&before)

        w := httptest.NewRecorder()
        ph.CreatePost(w, withUserID(newMultipartPostRequest(t, map[string]string{
            "content": "privacy test", "privacy_level": tc.level,
        }), author))
        if w.Code != tc.wantStatus {
            t.Errorf("privacy_level %q: status = %d, want %d (body %s)", tc.level, w.Code, tc.wantStatus, w.Body.String())
            continue
        }

        var after int
        db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&after)
        if tc.wantStatus != http.StatusCreated {
            if after != before {
                t.Errorf("privacy_level %q: a post was stored despite the rejection", tc.level)
            }
            continue
        }

        var stored string
        db.QueryRow("SELECT privacy_level FROM posts ORDER BY id DESC LIMIT 1").Scan(&stored)
        if stored != tc.wantStored {
            t.Errorf("privacy_level %q: stored %q, want %q", tc.level, stored, tc.wantStored)
        }
    }
}

// TestFollowRequestResponseMessages covers the message returned when a
// follow request is answered - "accepted" / "declined", as shown to the user.
func TestFollowRequestResponseMessages(t *testing.T) {
    db := newTestDB(t)
    user := &UserHandler{DB: db}
    owner := registerTestUser(t, db, "private-owner@example.com")
    if _, err := db.Exec("UPDATE users SET is_public = 0 WHERE id = ?", owner); err != nil {
        t.Fatalf("failed to make owner private: %v", err)
    }

    for _, tc := range []struct{ action, want string }{
        {"accept", "Follow request accepted"},
        {"decline", "Follow request declined"},
    } {
        follower := registerTestUser(t, db, tc.action+"-follower@example.com")
        user.FollowUser(httptest.NewRecorder(), withUserID(jsonRequest(t, http.MethodPost, "/api/user/follow",
            map[string]interface{}{"user_id": owner}), follower))

        w := httptest.NewRecorder()
        user.HandleFollowRequest(w, withUserID(jsonRequest(t, http.MethodPost, "/api/user/follow-request",
            map[string]interface{}{"follower_id": follower, "action": tc.action}), owner))
        if w.Code != http.StatusOK {
            t.Fatalf("%s: status = %d, body = %s", tc.action, w.Code, w.Body.String())
        }

        var resp map[string]string
        json.Unmarshal(w.Body.Bytes(), &resp)
        if resp["message"] != tc.want {
            t.Errorf("%s: message = %q, want %q", tc.action, resp["message"], tc.want)
        }
    }
}
