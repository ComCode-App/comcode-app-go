package upload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrUnsupportedType reports file bytes outside the allowlist for an upload.
var ErrUnsupportedType = errors.New("unsupported file type")

// imageTypes is the raster image allowlist with the extension the server
// stores. SVG is deliberately absent: it can carry script and is rejected.
var imageTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
	"image/gif":  "gif",
}

// Sniffed is a file whose type was detected from its bytes.
type Sniffed struct {
	// ContentType is the detected MIME type without parameters.
	ContentType string
	// Ext is the server-chosen extension (no dot); empty when not mapped.
	Ext string
	// Body replays the sniffed head followed by the rest of the file.
	Body io.Reader
}

// Sniff reads up to 512 bytes and detects the content type from them.
func Sniff(r io.Reader) (Sniffed, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return Sniffed{}, err
	}
	head = head[:n]
	ct := http.DetectContentType(head)
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	return Sniffed{ContentType: ct, Body: io.MultiReader(bytes.NewReader(head), r)}, nil
}

// SniffImage accepts only PNG, JPEG, WebP or GIF bytes. Any client-declared
// type or filename extension is ignored: the stored type and extension come
// from the bytes.
func SniffImage(r io.Reader) (Sniffed, error) {
	s, err := Sniff(r)
	if err != nil {
		return Sniffed{}, err
	}
	ext, ok := imageTypes[s.ContentType]
	if !ok {
		return Sniffed{}, fmt.Errorf("%w: images must be PNG, JPEG, WebP or GIF", ErrUnsupportedType)
	}
	s.Ext = ext
	return s, nil
}

// IsAllowedImageType reports whether a MIME type is in the raster image allowlist.
func IsAllowedImageType(ct string) bool {
	_, ok := imageTypes[normalize(ct)]
	return ok
}

// ImageExt returns the stored extension for an allowed image MIME type.
func ImageExt(ct string) (string, bool) {
	ext, ok := imageTypes[normalize(ct)]
	return ext, ok
}

func normalize(ct string) string {
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// SniffLogo accepts a raster image (as SniffImage) or a static SVG that
// passes ValidateSVG (owner decision: SVG is allowed for logos only). An SVG
// is read fully (up to MaxSVGBytes) to validate it; the returned Body replays
// it. Unsafe SVGs fail with ErrSVGUnsafe, anything else with ErrUnsupportedType.
func SniffLogo(r io.Reader) (Sniffed, error) {
	s, err := Sniff(r)
	if err != nil {
		return Sniffed{}, err
	}
	if ext, ok := imageTypes[s.ContentType]; ok {
		s.Ext = ext
		return s, nil
	}
	return sniffSVG(s, "logos must be PNG, JPEG, WebP, GIF or SVG")
}

// sniffSVG validates an SVG body (for uploads that accept vector artwork).
func sniffSVG(s Sniffed, unsupported string) (Sniffed, error) {
	data, err := io.ReadAll(io.LimitReader(s.Body, MaxSVGBytes+1))
	if err != nil {
		return Sniffed{}, err
	}
	if !LooksLikeSVG(data) {
		return Sniffed{}, fmt.Errorf("%w: %s", ErrUnsupportedType, unsupported)
	}
	if len(data) > MaxSVGBytes {
		return Sniffed{}, ErrTooLarge
	}
	if err := ValidateSVG(data); err != nil {
		return Sniffed{}, err
	}
	return Sniffed{ContentType: "image/svg+xml", Ext: "svg", Body: bytes.NewReader(data)}, nil
}

// artworkTypes are the non-image artwork types accepted for order assets.
var artworkTypes = map[string]string{
	"application/pdf":        "pdf",
	"application/postscript": "eps",
	"application/zip":        "zip",
}

// SniffArtwork accepts customer artwork for print orders: raster images,
// PDF (incl. PDF-compatible .ai), PostScript/EPS, ZIP, or a static SVG that
// passes ValidateSVG. Type and extension come from the bytes.
func SniffArtwork(r io.Reader) (Sniffed, error) {
	s, err := Sniff(r)
	if err != nil {
		return Sniffed{}, err
	}
	if ext, ok := imageTypes[s.ContentType]; ok {
		s.Ext = ext
		return s, nil
	}
	if ext, ok := artworkTypes[s.ContentType]; ok {
		s.Ext = ext
		return s, nil
	}
	return sniffSVG(s, "artwork must be PNG, JPEG, WebP, GIF, SVG, PDF, AI, EPS or ZIP")
}
