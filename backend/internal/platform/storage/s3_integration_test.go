package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
)

// TestS3CompatibleBackend exercises every Driver operation against a live
// S3-compatible store (SeaweedFS in compose). Skipped unless STORAGE_IT_ENDPOINT is set:
//
//	STORAGE_IT_ENDPOINT=http://127.0.0.1:8333 S3_ACCESS_KEY=... S3_SECRET_KEY=... S3_BUCKET=app \
//	  go test ./internal/platform/storage -run TestS3CompatibleBackend
func TestS3CompatibleBackend(t *testing.T) {
	endpoint := os.Getenv("STORAGE_IT_ENDPOINT")
	if endpoint == "" {
		t.Skip("STORAGE_IT_ENDPOINT not set")
	}
	bucket := envOr("S3_BUCKET", "app")
	cfg := config.ObjectStoreConfig{
		Endpoint:     endpoint,
		Region:       "us-east-1",
		Bucket:       bucket,
		AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		SecretKey:    os.Getenv("S3_SECRET_KEY"),
		UsePathStyle: true,
	}
	ctx := context.Background()
	d, err := NewS3(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	prefix := "it-" + time.Now().Format("20060102150405") + "/"
	key := prefix + "hello.txt"
	body := []byte("hello seaweed")
	t.Cleanup(func() {
		for _, k := range []string{key, prefix + "copy.txt", prefix + "presigned.txt"} {
			_ = d.Delete(context.Background(), k)
		}
	})

	if err := d.Upload(ctx, File{Body: bytes.NewReader(body), Size: int64(len(body)), ContentType: "text/plain", Metadata: map[string]string{"owner": "it"}}, key); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if ok, err := d.Exists(ctx, key); err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
	if ok, err := d.Exists(ctx, prefix+"missing.txt"); err != nil || ok {
		t.Fatalf("exists missing: %v %v", ok, err)
	}
	info, err := d.Head(ctx, key)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if info.Size != int64(len(body)) || !strings.HasPrefix(info.ContentType, "text/plain") || info.Metadata["owner"] != "it" {
		t.Fatalf("head info: %+v", info)
	}
	rc, size, err := d.Download(ctx, key)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if size != int64(len(body)) || !bytes.Equal(got, body) {
		t.Fatalf("download: size=%d body=%q", size, got)
	}

	if err := d.Copy(ctx, key, prefix+"copy.txt"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	page, err := d.List(ctx, ListObjectsInput{Prefix: prefix, Delimiter: "/"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Objects) != 2 {
		t.Fatalf("list: %+v", page.Objects)
	}
	root, err := d.List(ctx, ListObjectsInput{Delimiter: "/", MaxKeys: 1000})
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	foundPrefix := false
	for _, p := range root.Prefixes {
		foundPrefix = foundPrefix || p.Prefix == prefix
	}
	if !foundPrefix {
		t.Fatalf("list root prefixes missing %s: %+v", prefix, root.Prefixes)
	}
	versions, err := d.ListVersions(ctx, key)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) == 0 || !versions[0].IsLatest {
		t.Fatalf("versions: %+v", versions)
	}

	getURL, err := d.PresignGet(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("presign get: %v", err)
	}
	if b := httpBody(t, http.MethodGet, getURL, nil, ""); !bytes.Equal(b, body) {
		t.Fatalf("presigned get body %q", b)
	}
	putURL, err := d.PresignPut(ctx, prefix+"presigned.txt", "text/plain", time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}
	httpBody(t, http.MethodPut, putURL, []byte("via presign"), "text/plain")
	if ok, _ := d.Exists(ctx, prefix+"presigned.txt"); !ok {
		t.Fatal("presigned put did not store the object")
	}

	// Media URLs are served anonymously from the bucket (public read).
	if b := httpBody(t, http.MethodGet, d.PublicURL(key), nil, ""); !bytes.Equal(b, body) {
		t.Fatalf("public url body %q", b)
	}

	if err := d.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ok, _ := d.Exists(ctx, key); ok {
		t.Fatal("object still exists after delete")
	}
}

func httpBody(t *testing.T, method, url string, body []byte, contentType string) []byte {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = res.Body.Close() }()
	out, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %s", method, url, res.StatusCode, out)
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
