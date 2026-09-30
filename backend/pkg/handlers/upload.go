package handlers

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

var (
	ErrImageTooLarge    = errors.New("image too large")
	ErrInvalidImageType = errors.New("invalid image type")
)

// imageExtensionsByType maps a *sniffed* content type to the extension the
// file is saved under - the extension always reflects what the bytes
// actually are, never the client-supplied filename.
var imageExtensionsByType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
}

const uploadDir = "./uploads"

// saveUploadedImage validates and stores an uploaded file, and returns the
// public path (e.g. "/uploads/<uuid>.jpg") to store in the database.
//
// The type check sniffs the actual file bytes via http.DetectContentType
// rather than trusting the client-declared Content-Type header or the
// original filename's extension - both are attacker-controlled and easy to
// spoof (e.g. a non-image file renamed to "photo.jpg" with a forged
// Content-Type would pass a header-only check). Sniffing, plus deriving the
// saved extension from the sniffed type instead of the client's filename,
// means only files that are genuinely JPEG/PNG/GIF end up stored - and
// under an extension that matches what they really are.
func saveUploadedImage(file multipart.File, header *multipart.FileHeader, maxSize int64) (string, error) {
	if header.Size > maxSize {
		return "", ErrImageTooLarge
	}

	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && err != io.EOF {
		return "", err
	}
	detected := http.DetectContentType(sniff[:n])

	ext, ok := imageExtensionsByType[detected]
	if !ok {
		return "", ErrInvalidImageType
	}

	// Detection only consumed the first 512 bytes - rewind before copying
	// the full file to disk.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", err
	}

	filename := uuid.New().String() + ext
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}

	return "/uploads/" + filename, nil
}

// deleteUploadedImage removes a previously stored image given its public
// path (as stored in the DB, e.g. "/uploads/<uuid>.jpg"). Missing files or
// an empty path are not errors - there's simply nothing to clean up.
func deleteUploadedImage(publicPath string) {
	if publicPath == "" {
		return
	}
	filename := filepath.Base(publicPath)
	os.Remove(filepath.Join(uploadDir, filename))
}

// imageUploadErrorResponse writes the right status/message for a
// saveUploadedImage error.
func imageUploadErrorResponse(w http.ResponseWriter, err error) {
	switch err {
	case ErrImageTooLarge:
		writeJSONError(w, http.StatusBadRequest, "Image size must be less than 5MB")
	case ErrInvalidImageType:
		writeJSONError(w, http.StatusBadRequest, "Invalid image type. Only JPEG, PNG, and GIF are allowed")
	default:
		writeJSONError(w, http.StatusInternalServerError, "Failed to save image")
	}
}
