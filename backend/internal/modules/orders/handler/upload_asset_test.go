package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
)

const goodToken = "good-token"

// fakeRepo answers the asset-token lookups; every other method panics via the nil embed.
type fakeRepo struct {
	repository.Repository
	assetID  string
	uploaded string
}

func (f *fakeRepo) GetAssetRequestByToken(_ context.Context, token string) (*model.AssetRequestSummary, int64, string, error) {
	if token != goodToken {
		return nil, 0, "", nil
	}
	return &model.AssetRequestSummary{UUID: f.assetID, Status: "pending"}, 1, "ORD-1", nil
}

func (f *fakeRepo) UpdateAssetRequestStatus(_ context.Context, _ uuid.UUID, status, fileURL, _ string) (*model.AssetRequestSummary, error) {
	f.uploaded = fileURL
	return &model.AssetRequestSummary{UUID: f.assetID, Status: status, UploadedFileURL: fileURL}, nil
}

type memStore struct {
	storage.Driver
	mem *storage.Memory
}

func (s memStore) Upload(ctx context.Context, f storage.File, key string) error {
	return s.mem.Upload(ctx, f, key)
}
func (s memStore) PublicURL(key string) string { return "/api/v1/storage/stream/" + key }

func newAssetHandler() (*Handler, *fakeRepo, *storage.Memory) {
	repo := &fakeRepo{assetID: uuid.NewString()}
	mem := storage.NewMemory()
	return New(usecase.NewService(repo, nil, nil, memStore{mem: mem}, "")), repo, mem
}

// countingReader records how much of the request body the handler consumed.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

const boundary = "XBOUNDARYX"

// assetBody is a multipart body with the token field first and a file of fileSize bytes.
func assetBody(token, filename string, file io.Reader) io.Reader {
	head := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"token\"\r\n\r\n%s\r\n"+
		"--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: image/png\r\n\r\n",
		boundary, token, boundary, filename)
	tail := fmt.Sprintf("\r\n--%s--\r\n", boundary)
	return io.MultiReader(strings.NewReader(head), file, strings.NewReader(tail))
}

func postAsset(h *Handler, body io.Reader, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/orders/assets/upload"+query, body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	rec := httptest.NewRecorder()
	h.UploadAsset(rec, req)
	return rec
}

// An invalid token is rejected before the (unauthenticated) upload is read.
func TestUploadAssetBadTokenDoesNotConsumeBody(t *testing.T) {
	h, _, _ := newAssetHandler()
	body := &countingReader{r: assetBody("wrong", "a.png", io.LimitReader(zeros{}, 200<<20))}
	rec := postAsset(h, body, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if body.n > 64<<10 {
		t.Fatalf("handler read %d bytes of the body before rejecting the token", body.n)
	}

	// Same through ?token=: nothing of the body is needed.
	body = &countingReader{r: assetBody("ignored", "a.png", io.LimitReader(zeros{}, 200<<20))}
	if rec := postAsset(h, body, "?token=wrong"); rec.Code != http.StatusNotFound || body.n > 64<<10 {
		t.Fatalf("query token: status %d, read %d", rec.Code, body.n)
	}
}

func TestUploadAssetOversizedBodyIs413(t *testing.T) {
	h, _, _ := newAssetHandler()
	rec := postAsset(h, assetBody(goodToken, "a.png", io.LimitReader(zeros{}, maxOrderAssetBytes+(2<<20))), "")
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestUploadAssetRejectsHTMLDisguisedAsPNG(t *testing.T) {
	h, repo, _ := newAssetHandler()
	rec := postAsset(h, assetBody(goodToken, "logo.png", strings.NewReader("<html><script>alert(1)</script></html>")), "")
	if rec.Code != http.StatusBadRequest || repo.uploaded != "" {
		t.Fatalf("status %d %s (uploaded %q)", rec.Code, rec.Body.String(), repo.uploaded)
	}
	rec = postAsset(h, assetBody(goodToken, "logo.svg", strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)), "")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "SVG_UNSAFE_CONTENT") {
		t.Fatalf("unsafe svg: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUploadAssetStoresSniffedType(t *testing.T) {
	h, repo, mem := newAssetHandler()
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	// The filename claims .html; the bytes are a PNG.
	rec := postAsset(h, assetBody(goodToken, "x.html", bytes.NewReader(png)), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	key := "orders/ORD-1/assets/" + repo.assetID + ".png"
	if !strings.HasSuffix(repo.uploaded, key) {
		t.Fatalf("stored at %q, want suffix %q", repo.uploaded, key)
	}
	if ct, _, ok := mem.GetMeta(key); !ok || ct != "image/png" {
		t.Fatalf("stored content type %q (%v)", ct, ok)
	}
}

func TestUploadAssetNeedsTokenBeforeFile(t *testing.T) {
	h, _, _ := newAssetHandler()
	body := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\n\r\nx\r\n--%s--\r\n", boundary, boundary)
	if rec := postAsset(h, strings.NewReader(body), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}
