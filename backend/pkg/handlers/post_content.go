package handlers

import (
    "net/http"
    "strings"
    "unicode/utf8"
)

// hasImageUpload reports whether an already-parsed multipart form carries
// a file in its "image" field. Whether that file really is a JPG, PNG or
// GIF is still decided by saveUploadedImage.
func hasImageUpload(r *http.Request) bool {
    return r.MultipartForm != nil && len(r.MultipartForm.File["image"]) > 0
}

// contentError applies the rule shared by posts and comments: they need
// text, an image, or both. It returns the message to show the user, or ""
// if the content is fine.
//
// Any valid UTF-8 text is accepted exactly as typed - letters in any
// script (ASCII, French accents, Cyrillic, CJK ideographs, Hiragana,
// Katakana, Hangul...), digits, punctuation, symbols, emoji, spaces,
// newlines and other control characters. Nothing is filtered or rewritten;
// only text made up entirely of whitespace counts as empty.
func contentError(content string, hasImage bool) string {
    if !utf8.ValidString(content) {
        return "Text must be valid UTF-8"
    }
    if strings.TrimSpace(content) == "" && !hasImage {
        return "Write something or add an image (JPG, PNG or GIF)"
    }
    return ""
}
