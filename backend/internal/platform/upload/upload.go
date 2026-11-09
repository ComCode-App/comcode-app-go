// Package upload is the shared guard for multipart file uploads: it caps the
// request body before anything reads it (http.MaxBytesReader), maps an
// oversized body to 413 PAYLOAD_TOO_LARGE, and sniffs file bytes so the
// stored content type and extension never come from the client.
package upload

import (
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// MultipartOverhead is slack added on top of a file limit for the multipart
// envelope (boundaries, part headers, small form fields), so a file at exactly
// the limit is not rejected by framing bytes alone. The exact per-file limit
// is enforced on the parsed part size.
const MultipartOverhead = 1 << 20

// Per-use file limits. Image uploads get a small cap; STORAGE_MAX_BYTES bounds
// the generic storage explorer upload.
const (
	MaxImageBytes = 5 << 20
	// DefaultMaxBytes is used when a non-positive limit is configured.
	DefaultMaxBytes = 10 << 20
	// maxMemory bounds what ParseMultipartForm keeps in memory; the rest of
	// an (already capped) body spills to temp files.
	maxMemory = 4 << 20
)

// ErrTooLarge reports a file above its per-use limit.
var ErrTooLarge = errors.New("upload exceeds the maximum allowed size")

// LimitBody caps r.Body at maxFileBytes plus the multipart envelope slack.
// Call it before anything reads the body (FormValue, ParseMultipartForm,
// MultipartReader).
func LimitBody(w http.ResponseWriter, r *http.Request, maxFileBytes int64) {
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultMaxBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes+MultipartOverhead)
}

// IsTooLarge reports whether err comes from a capped body or a file above its limit.
func IsTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr) || errors.Is(err, ErrTooLarge)
}

// WriteTooLarge writes the 413 PAYLOAD_TOO_LARGE envelope.
func WriteTooLarge(w http.ResponseWriter, r *http.Request) {
	response.PayloadTooLarge(w, r, ErrTooLarge.Error())
}

// ParseForm caps the body and parses a multipart form. On failure it writes
// 413 (body over the cap) or 400 (malformed form) and returns false. Call
// Cleanup when done to remove spilled temp files.
func ParseForm(w http.ResponseWriter, r *http.Request, maxFileBytes int64) bool {
	LimitBody(w, r, maxFileBytes)
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		if IsTooLarge(err) {
			WriteTooLarge(w, r)
			return false
		}
		response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
		return false
	}
	return true
}

// Cleanup removes temp files ParseMultipartForm may have written.
func Cleanup(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

// FormFile caps the body, parses the form and returns the first file found in
// fields (in order). It writes 413/400 itself and returns ok=false on any
// failure, including a file above maxFileBytes. The caller closes the file
// and calls Cleanup.
func FormFile(w http.ResponseWriter, r *http.Request, maxFileBytes int64, fields ...string) (multipart.File, *multipart.FileHeader, bool) {
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultMaxBytes
	}
	if !ParseForm(w, r, maxFileBytes) {
		return nil, nil, false
	}
	for _, field := range fields {
		file, header, err := r.FormFile(field)
		if err != nil {
			continue
		}
		if header.Size > maxFileBytes {
			_ = file.Close()
			WriteTooLarge(w, r)
			return nil, nil, false
		}
		return file, header, true
	}
	name := "file"
	if len(fields) > 0 {
		name = fields[0]
	}
	response.BadRequest(w, r, response.CodeValidationError, name+" file is required")
	return nil, nil, false
}

// WriteError maps upload errors (too large, unsafe SVG, unsupported type) to their
// envelope and reports whether it handled err.
func WriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case IsTooLarge(err):
		WriteTooLarge(w, r)
		return true
	case errors.Is(err, ErrSVGUnsafe):
		reason := err.Error()
		var se *SVGUnsafeError
		if errors.As(err, &se) {
			reason = se.Reason
		}
		response.ErrorWithDetails(w, r, http.StatusUnprocessableEntity, response.CodeSVGUnsafeContent,
			"SVG contains unsafe content (script, event handlers or external references) and was not uploaded",
			[]response.Detail{{Field: "file", Message: reason, Code: "svg_unsafe"}})
		return true
	case errors.Is(err, ErrUnsupportedType):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return true
	}
	return false
}
