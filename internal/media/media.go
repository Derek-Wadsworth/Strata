package media

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
)

var imageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
}

// IsImagePath reports whether path looks like a supported image by extension.
func IsImagePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := imageExts[ext]
	return ok
}

// MIME returns a content type for path, preferring extension then sniffing bytes.
func MIME(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := imageExts[ext]; ok {
		return mime
	}
	if len(data) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(data)
}

// IsImageData reports whether data looks like a common image format.
func IsImageData(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	switch {
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}):
		return true // PNG
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		return true // JPEG
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return true
	case bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")):
		return true
	case bytes.HasPrefix(data, []byte("BM")):
		return true
	default:
		return false
	}
}
