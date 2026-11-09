package upload_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
)

var pngHead = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func multipartBody(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, rec.Body.String())
	}
	if env.Success {
		t.Fatal("expected an error envelope")
	}
	return env.Error.Code
}

func TestFormFileRejectsOversizedBodyWith413(t *testing.T) {
	const limit = 1 << 10
	// The body is far above limit+overhead: MaxBytesReader stops it.
	body, ct := multipartBody(t, "file", "big.bin", bytes.Repeat([]byte("a"), limit+upload.MultipartOverhead+1))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	if _, _, ok := upload.FormFile(rec, req, limit, "file"); ok {
		t.Fatal("oversized body accepted")
	}
	if rec.Code != http.StatusRequestEntityTooLarge || errorCode(t, rec) != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestFormFileRejectsFileAboveExactLimit(t *testing.T) {
	const limit = 1 << 10
	// Within the envelope slack but above the per-file limit.
	body, ct := multipartBody(t, "file", "f.bin", bytes.Repeat([]byte("a"), limit+10))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	if _, _, ok := upload.FormFile(rec, req, limit, "file"); ok {
		t.Fatal("file above limit accepted")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestFormFileAcceptsFileWithinLimit(t *testing.T) {
	body, ct := multipartBody(t, "logo", "a.png", pngHead)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	file, header, ok := upload.FormFile(rec, req, 1<<10, "logo")
	defer upload.Cleanup(req)
	if !ok {
		t.Fatalf("rejected: %d %s", rec.Code, rec.Body.String())
	}
	defer func() { _ = file.Close() }()
	if header.Size != int64(len(pngHead)) {
		t.Fatalf("size %d", header.Size)
	}
}

func TestSniffImageRejectsHTMLDisguisedAsPNG(t *testing.T) {
	// Name/Content-Type say PNG; the bytes are HTML. Only the bytes count.
	if _, err := upload.SniffImage(strings.NewReader("<!doctype html><html><script>alert(1)</script></html>")); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("html accepted as image: %v", err)
	}
	if _, err := upload.SniffImage(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"/>`)); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("svg accepted as raster image: %v", err)
	}
	s, err := upload.SniffImage(bytes.NewReader(pngHead))
	if err != nil || s.ContentType != "image/png" || s.Ext != "png" {
		t.Fatalf("png: %+v %v", s, err)
	}
	replayed, _ := io.ReadAll(s.Body)
	if !bytes.Equal(replayed, pngHead) {
		t.Fatal("sniffed body does not replay the head")
	}
}

func TestSniffArtwork(t *testing.T) {
	cases := map[string]string{
		"%PDF-1.4\n%%EOF":   "pdf",
		"%!PS-Adobe-3.0":    "eps",
		"PK\x03\x04rest...": "zip",
		string(pngHead):     "png",
		`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h1"/></svg>`: "svg",
	}
	for body, ext := range cases {
		s, err := upload.SniffArtwork(strings.NewReader(body))
		if err != nil || s.Ext != ext {
			t.Errorf("%q: got %+v %v, want %s", body[:4], s, err, ext)
		}
	}
	if _, err := upload.SniffArtwork(strings.NewReader("<html><body>x</body></html>")); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("html artwork: %v", err)
	}
	if _, err := upload.SniffArtwork(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"><script>x</script></svg>`)); !errors.Is(err, upload.ErrSVGUnsafe) {
		t.Fatalf("unsafe svg artwork: %v", err)
	}
}

func TestSetServeHeaders(t *testing.T) {
	cases := []struct {
		ct, name    string
		download    bool
		disposition string
		csp         bool
	}{
		{"image/png", "a.png", false, `inline; filename="a.png"`, false},
		{"application/pdf", "a.pdf", false, `inline; filename="a.pdf"`, false},
		{"video/mp4", "a.mp4", false, `inline; filename="a.mp4"`, false},
		{"image/png", "a.png", true, `attachment; filename="a.png"`, false},
		{"text/html; charset=utf-8", "avatar.html", false, `attachment; filename="avatar.html"`, true},
		{"image/svg+xml", "logo.svg", false, `attachment; filename="logo.svg"`, true},
		{"application/xml", "x.xml", false, `attachment; filename="x.xml"`, true},
		{"text/xml", "x.xml", false, `attachment; filename="x.xml"`, true},
		{"application/javascript", "x.js", false, `attachment; filename="x.js"`, true},
		{"", "x", false, `attachment; filename="x"`, true},
		{"image/png", "a\"\r\nb.png", false, `inline; filename="ab.png"`, false},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		upload.SetServeHeaders(rec, c.ct, c.name, c.download)
		h := rec.Header()
		if h.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", c.ct)
		}
		if got := h.Get("Content-Disposition"); got != c.disposition {
			t.Errorf("%s: disposition %q, want %q", c.ct, got, c.disposition)
		}
		if got := h.Get("Content-Security-Policy"); (got != "") != c.csp {
			t.Errorf("%s: csp %q", c.ct, got)
		}
	}
	rec := httptest.NewRecorder()
	upload.SetServeHeaders(rec, "image/svg+xml", "logo.svg", false)
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'; sandbox" {
		t.Fatalf("svg csp %q", got)
	}
	if rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("svg type %q", rec.Header().Get("Content-Type"))
	}
}

func TestContentTypeForKey(t *testing.T) {
	for key, want := range map[string]string{
		"profiles/x/avatar.PNG":  "image/png",
		"profiles/x/avatar.html": "text/html; charset=utf-8",
		"logo.svg":               "image/svg+xml",
		"noext":                  "application/octet-stream",
	} {
		if got := upload.ContentTypeForKey(key); got != want {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
	}
}
