package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage/model"
	storageusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage/usecase"
	platstorage "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

// memDriver adapts the in-memory store; methods StreamMedia does not use panic via the nil embed.
type memDriver struct {
	platstorage.Driver
	mem *platstorage.Memory
}

func (d memDriver) Download(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	return d.mem.Download(ctx, key)
}

// A file stored as avatar.html must never render as HTML on our origin.
func TestStreamMediaServesActiveContentAsAttachment(t *testing.T) {
	mem := platstorage.NewMemory()
	ctx := context.Background()
	for key, body := range map[string]string{
		"profiles/p/avatar.html": "<script>alert(document.cookie)</script>",
		"profiles/p/logo.svg":    `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"profiles/p/avatar.png":  "\x89PNG\r\n\x1a\n",
	} {
		if err := mem.Upload(ctx, platstorage.File{Body: strings.NewReader(body), ContentType: "text/html"}, key); err != nil {
			t.Fatal(err)
		}
	}
	h := New(storageusecase.New(memDriver{mem: mem}, nil, nil), 0)

	cases := []struct {
		key, ct, disp string
	}{
		{"profiles/p/avatar.html", "text/html; charset=utf-8", "attachment"},
		{"profiles/p/logo.svg", "image/svg+xml", "attachment"},
		{"profiles/p/avatar.png", "image/png", "inline"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/v1/storage/stream/"+c.key, nil)
		req.SetPathValue("key", c.key)
		rec := httptest.NewRecorder()
		h.StreamMedia(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", c.key, rec.Code)
		}
		hd := rec.Header()
		if hd.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", c.key)
		}
		if hd.Get("Content-Type") != c.ct {
			t.Errorf("%s: content type %q", c.key, hd.Get("Content-Type"))
		}
		if !strings.HasPrefix(hd.Get("Content-Disposition"), c.disp) {
			t.Errorf("%s: disposition %q, want %s", c.key, hd.Get("Content-Disposition"), c.disp)
		}
		if c.disp == "attachment" && !strings.Contains(hd.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s: missing sandbox CSP", c.key)
		}
	}
}

// Preview/public links use the stored (uploader-declared) MIME: HTML must download.
func TestWriteFileNeverServesHTMLInline(t *testing.T) {
	rec := httptest.NewRecorder()
	writeFile(rec, model.OpenObject{
		Object: model.Object{Name: "avatar.html", MimeType: "text/html"},
		Body:   io.NopCloser(strings.NewReader("<script>alert(1)</script>")),
		Size:   25,
	})
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="avatar.html"` {
		t.Fatalf("disposition %q", got)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}

	rec = httptest.NewRecorder()
	writeFile(rec, model.OpenObject{
		Object: model.Object{Name: "a.pdf", MimeType: "application/pdf"},
		Body:   io.NopCloser(strings.NewReader("%PDF")),
	})
	if got := rec.Header().Get("Content-Disposition"); got != `inline; filename="a.pdf"` {
		t.Fatalf("pdf disposition %q", got)
	}
}

func TestUploadRejectsBodyAboveStorageMaxBytes(t *testing.T) {
	const maxBytes = 4 << 10
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "big.bin")
	_, _ = fw.Write(bytes.Repeat([]byte("x"), maxBytes+(1<<20)+1))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/platform/storage/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	// The svc is never reached: the cap rejects the body first.
	New(nil, maxBytes).Upload(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}
