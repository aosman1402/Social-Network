package handlers

import (
    "bytes"
    "image"
    "image/png"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "strconv"
    "testing"
)

// allScriptsText exercises every kind of character a post or comment must
// accept: Latin, French accents and Cyrillic letters in both cases, digits,
// punctuation and symbols, spaces/newlines/tabs, CJK ideographs, Hiragana,
// Katakana, Hangul, and emoji (including a ZWJ sequence and a flag).
const allScriptsText = "Hello World - Ça va? Élève où, Привет МИР\n" +
    "0123456789 ٣\n" +
    "!\"#%&'()*,-./:;?@[]_{}«»¿¡… $+<=>^`|~€£©®°±\n" +
    "tab\there\r\nspaces   end\n" +
    "漢字 ひらがな カタカナ 한국어\n" +
    "😀🎉❤️👍🏽👨‍👩‍👧🇫🇷"

// newMultipartImageRequest builds a multipart request with the given form
// fields plus a real 1x1 PNG in the "image" field.
func newMultipartImageRequest(t *testing.T, url string, fields map[string]string) *http.Request {
    t.Helper()
    var buf bytes.Buffer
    mw := multipart.NewWriter(&buf)
    for k, v := range fields {
        mw.WriteField(k, v)
    }
    fw, err := mw.CreateFormFile("image", "pixel.png")
    if err != nil {
        t.Fatalf("CreateFormFile: %v", err)
    }
    if err := png.Encode(fw, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
        t.Fatalf("png.Encode: %v", err)
    }
    mw.Close()
    r := httptest.NewRequest(http.MethodPost, url, &buf)
    r.Header.Set("Content-Type", mw.FormDataContentType())
    return r
}

// removeUploadedImageFor deletes the file an image test saved to disk.
func removeUploadedImageFor(t *testing.T, ph *PostHandler, query string, id int64) {
    t.Cleanup(func() {
        var path string
        ph.DB.QueryRow(query, id).Scan(&path)
        deleteUploadedImage(path)
    })
}

func TestPostAndCommentContentRules(t *testing.T) {
    db := newTestDB(t)
    ph := &PostHandler{DB: db}
    author := registerTestUser(t, db, "content-author@example.com")

    createPost := func(r *http.Request) (int, int64) {
        w := httptest.NewRecorder()
        ph.CreatePost(w, withUserID(r, author))
        var id int64
        db.QueryRow("SELECT MAX(id) FROM posts").Scan(&id)
        return w.Code, id
    }

    t.Run("text in every script is stored exactly as typed", func(t *testing.T) {
        code, postID := createPost(newMultipartPostRequest(t, map[string]string{"content": allScriptsText}))
        if code != http.StatusCreated {
            t.Fatalf("CreatePost status = %d, want 201", code)
        }
        var stored string
        db.QueryRow("SELECT content FROM posts WHERE id = ?", postID).Scan(&stored)
        if stored != allScriptsText {
            t.Errorf("post content changed:\n got %q\nwant %q", stored, allScriptsText)
        }

        w := httptest.NewRecorder()
        ph.CreateComment(w, withUserID(newMultipartPostRequest(t, map[string]string{
            "content": allScriptsText, "post_id": strconv.FormatInt(postID, 10),
        }), author))
        if w.Code != http.StatusOK && w.Code != http.StatusCreated {
            t.Fatalf("CreateComment status = %d, body = %s", w.Code, w.Body.String())
        }
        db.QueryRow("SELECT content FROM comments WHERE post_id = ?", postID).Scan(&stored)
        if stored != allScriptsText {
            t.Errorf("comment content changed:\n got %q\nwant %q", stored, allScriptsText)
        }
    })

    t.Run("image with no text is accepted for posts and comments", func(t *testing.T) {
        code, postID := createPost(newMultipartImageRequest(t, "/api/post/create", nil))
        removeUploadedImageFor(t, ph, "SELECT image_path FROM posts WHERE id = ?", postID)
        if code != http.StatusCreated {
            t.Fatalf("image-only CreatePost status = %d, want 201", code)
        }

        w := httptest.NewRecorder()
        ph.CreateComment(w, withUserID(newMultipartImageRequest(t, "/api/comment/create", map[string]string{
            "post_id": strconv.FormatInt(postID, 10),
        }), author))
        removeUploadedImageFor(t, ph, "SELECT image_path FROM comments WHERE post_id = ?", postID)
        if w.Code != http.StatusOK && w.Code != http.StatusCreated {
            t.Fatalf("image-only CreateComment status = %d, body = %s", w.Code, w.Body.String())
        }
    })

    t.Run("no text and no image is rejected", func(t *testing.T) {
        for _, content := range []string{"", "   \n\t　"} {
            if code, _ := createPost(newMultipartPostRequest(t, map[string]string{"content": content})); code != http.StatusBadRequest {
                t.Errorf("CreatePost(%q) status = %d, want 400", content, code)
            }
        }
    })

    t.Run("invalid UTF-8 is rejected", func(t *testing.T) {
        if code, _ := createPost(newMultipartPostRequest(t, map[string]string{"content": "bad \xff\xfe bytes"})); code != http.StatusBadRequest {
            t.Errorf("CreatePost(invalid UTF-8) status = %d, want 400", code)
        }
    })
}
