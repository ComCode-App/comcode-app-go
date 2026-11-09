package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

// memDriver adapts the in-memory store; unused Driver methods panic via the nil embed.
type memDriver struct {
	storage.Driver
	mem *storage.Memory
}

func (d memDriver) Upload(ctx context.Context, f storage.File, key string) error {
	return d.mem.Upload(ctx, f, key)
}

func postLogo(t *testing.T, h *Handler, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("logo", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/platform/settings/logo", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadLogo(rec, req)
	return rec
}

// TEC-1097: rejected logos never reach storage or the settings service.
func TestUploadLogoRejectsUnsafeContent(t *testing.T) {
	mem := storage.NewMemory()
	h := New(nil, memDriver{mem: mem}, nil)

	rec := postLogo(t, h, "logo.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "SVG_UNSAFE_CONTENT") {
		t.Fatalf("unsafe svg: %d %s", rec.Code, rec.Body.String())
	}
	rec = postLogo(t, h, "logo.png", []byte("<html><script>alert(1)</script></html>"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("html as png: %d %s", rec.Code, rec.Body.String())
	}
	rec = postLogo(t, h, "logo.png", bytes.Repeat([]byte("a"), storage.MaxLogoBytes+(2<<20)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized logo: %d", rec.Code)
	}
	for _, key := range []string{storage.AppLogoObjectKey("svg"), storage.AppLogoObjectKey("png")} {
		if ok, _ := mem.Exists(t.Context(), key); ok {
			t.Fatalf("%s was stored", key)
		}
	}
}
