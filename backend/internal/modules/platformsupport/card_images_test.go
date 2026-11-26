package platformsupport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 5))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (f fixture) upload(t *testing.T, side string, body []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("image", "design.bin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(body)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPut, "/v1/platform/cards/"+cardUUID.String()+"/images/"+side, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Perms", rbac.PermPlatformCardsWrite)
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env
}

func TestCardImageUploadAndRemove(t *testing.T) {
	f := newFixture(t)
	mem := storage.NewMemory()
	f.svc.SetStorage(mem)
	if _, err := f.store.SetGrant(context.Background(), staffUUID, "password", time.Hour); err != nil {
		t.Fatal(err)
	}

	if status, _ := f.upload(t, "front", pngBytes(t)); status != http.StatusOK {
		t.Fatalf("upload front: %d", status)
	}
	first := f.repo.front
	if !strings.Contains(first, "cards/"+cardUUID.String()+"/front-") || !strings.HasSuffix(first, ".png") {
		t.Fatalf("front url = %q", first)
	}
	// Replacing the image deletes the previous object.
	time.Sleep(2 * time.Millisecond)
	if status, _ := f.upload(t, "front", pngBytes(t)); status != http.StatusOK {
		t.Fatal("replace front failed")
	}
	if f.repo.front == first {
		t.Fatal("replaced image kept the same url")
	}
	key := first[strings.Index(first, "cards/"):]
	if ok, _ := mem.Exists(context.Background(), key); ok {
		t.Fatal("replaced object was not deleted")
	}

	// Only PNG, JPEG and WebP: GIF and non-images are rejected.
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	if status, _ := f.upload(t, "back", gif); status != http.StatusBadRequest && status != http.StatusUnsupportedMediaType {
		t.Fatalf("gif upload status = %d", status)
	}
	if status, _ := f.upload(t, "back", []byte("<html>")); status == http.StatusOK {
		t.Fatal("html accepted as card image")
	}
	if status, _ := f.upload(t, "side", pngBytes(t)); status != http.StatusBadRequest {
		t.Fatalf("invalid side status = %d", status)
	}
	if f.repo.back != "" {
		t.Fatal("rejected uploads changed the back image")
	}

	if status, _ := f.do(http.MethodDelete, "/v1/platform/cards/"+cardUUID.String()+"/images/front", rbac.PermPlatformCardsWrite, ""); status != http.StatusOK {
		t.Fatalf("remove front: %d", status)
	}
	if f.repo.front != "" {
		t.Fatal("front image not cleared")
	}
}
