package legacymigration

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestRepairAndTransform(t *testing.T) {
	for in, want := range map[string]string{"Ä°nternet Sitemi": "İnternet Sitemi", "Åžirket HesabÄ±": "Şirket Hesabı", "CV&#39;mi": "CV'mi", "İnternet Şirketi": "İnternet Şirketi", "普通话": "普通话"} {
		if got := RepairText(in); got != want {
			t.Errorf("repair %q = %q, want %q", in, got, want)
		}
	}
	source := LegacyUser{"user_id": "7", "uniqid_id": "remax_1", "username": "owner", "name": "Name", "active": "1", "deactivated": "1", "logo4": "AR Teknoloji,Teknoloji,Pazarlama", "firmalogo": "logo.png", "user_card_img": "template/images/shared.png", "user_card_img_back": "usercard/back.png", "buttons": `{"instagram":"owner","Location":"İstanbul","Mystery":"https://example.com","empty":"","bad":"javascript:alert(1)"}`, "template": "two", "vcard": `{"firstname":"CV&#39;mi &quot;Name&quot;"}`, "password": "NEVER_COPY", "vcard_fr": `{"firstname":"Jean"}`}
	got, err := Transform(source, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.Translations[0].FirstName != `CV'mi "Name"` {
		t.Fatal("JSON entity repair lost names")
	}
	if got.Published || len(got.Media) != 2 || got.Metadata["logo4"] != source["logo4"] || got.Metadata["template"] != "compact_card" {
		t.Fatalf("unexpected mapping: %+v", got)
	}
	for _, link := range got.Profile.SocialLinks {
		if link.Label == "instagram" && (link.Platform != "instagram" || link.URL != "https://instagram.com/owner") {
			t.Fatal(link)
		}
		if link.Label == "bad" && link.IsActive {
			t.Fatal("unsafe link active")
		}
	}
	if strings.Contains(string(got.Row.ExtraJSON), "NEVER_COPY") {
		t.Fatal("secret copied")
	}
	changed := source
	changed["password"] = "different"
	again, err := Transform(changed, map[string]string{})
	if err != nil || again.ContentHash != got.ContentHash {
		t.Fatal("secret affected hash")
	}
}

func TestMediaDedupeAndTraversal(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "public_html", "uploads", "avatars")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.png"), []byte("same bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.png"), []byte("same bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	m := &MediaSync{Dir: dir, Driver: storage.NewMemory()}
	ctx := context.Background()
	if err := m.Load(ctx); err != nil {
		t.Fatal(err)
	}
	a, err := m.Put(ctx, "profile", MediaRef{Field: "avatar", Path: "uploads/avatars/a.png"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Put(ctx, "profile", MediaRef{Field: "cover", Path: "uploads/avatars/b.png"})
	if err != nil || a != b || m.Count != 1 {
		t.Fatalf("dedupe: %q %q count=%d err=%v", a, b, m.Count, err)
	}
	if _, err := localMedia(dir, "uploads/avatars/../../secret.png"); err == nil {
		t.Fatal("traversal allowed")
	}
	if err := os.Symlink(filepath.Join(root, "a.png"), filepath.Join(root, "link.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := localMedia(dir, "uploads/avatars/link.png"); err == nil {
		t.Fatal("symlink allowed")
	}
}

func TestExporterClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "secret" || r.URL.Query().Get("mode") != "dump" {
			t.Error("wrong exporter query")
		}
		z := gzip.NewWriter(w)
		_, err := io.WriteString(z, "user_id,uniqid_id\n1,abcde\n")
		if err != nil {
			t.Error(err)
		}
		if err := z.Close(); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := &ExportClient{URL: server.URL, Token: "secret"}
	count := 0
	if err := readCSV(context.Background(), c, "", "users", "", func(row map[string]string) error {
		count++
		if row["uniqid_id"] != "abcde" {
			t.Error(row)
		}
		return nil
	}); err != nil || count != 1 {
		t.Fatalf("client count=%d err=%v", count, err)
	}
	server.Close()
	_, err := c.Open(context.Background(), "dump", nil)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("transport error exposes token")
	}
}

func TestSyncIdempotencyRealPostgres(t *testing.T) {
	d := pgtest.Open(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "users.csv")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(f)
	for _, row := range [][]string{{"user_id", "uniqid_id", "username", "name", "active", "deactivated", "bank_accounts"}, {"sync-560-1", "sync560hash", "sync560owner", "Ä°nternet Sitemi", "1", "2", `[{"bank":"Bank","iban":"TR123","account_name":"Owner"}]`}, {"sync-560-2", "sync560inactive", "sync560inactiveowner", "Inactive", "1", "1", ""}} {
		if err = w.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	opts := SyncOptions{Options: options(), UsersCSV: source, StateDir: filepath.Join(dir, "state"), Media: &MediaSync{Driver: storage.NewMemory()}}
	repo := NewPostgres(d.Pool)
	ctx := context.Background()
	var report bytes.Buffer
	opts.DryRun = true
	planned, err := Sync(ctx, repo, opts, &report)
	if err != nil || planned.Creates != 2 {
		t.Fatalf("plan %+v: %v", planned, err)
	}
	if _, err = os.Stat(opts.StateDir); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote state")
	}
	opts.DryRun = false
	first, err := Sync(ctx, repo, opts, &report)
	if err != nil || first.Creates != 2 {
		t.Fatalf("first %+v: %v", first, err)
	}
	opts.Media = &MediaSync{Driver: storage.NewMemory()}
	second, err := Sync(ctx, repo, opts, &report)
	if err != nil || second.Skips != 2 || second.Creates != 0 || second.Updates != 0 {
		t.Fatalf("second %+v: %v", second, err)
	}
	before := []byte{}
	if err = d.Pool.QueryRow(ctx, `SELECT legacy_metadata FROM routes WHERE path='sync560hash'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("Ä°nternet Sitemi"), []byte("Updated Name"))
	if err = os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	updated, err := Sync(ctx, repo, opts, &report)
	if err != nil || updated.Updates != 1 || updated.Skips != 1 {
		t.Fatalf("update %+v err=%v", updated, err)
	}
	after := []byte{}
	if err = d.Pool.QueryRow(ctx, `SELECT legacy_metadata FROM routes WHERE path='sync560hash'`).Scan(&after); err != nil || !bytes.Equal(before, after) {
		t.Fatal("immutable provenance changed", err)
	}
	if err = os.Remove(filepath.Join(opts.StateDir, "state.json")); err != nil {
		t.Fatal(err)
	}
	recovered, err := Sync(ctx, repo, opts, &report)
	if err != nil || recovered.Skips != 2 {
		t.Fatalf("state-loss recovery %+v err=%v", recovered, err)
	}

	var active bool
	if err = d.Pool.QueryRow(ctx, `SELECT status='locked' FROM routes WHERE path='sync560inactive'`).Scan(&active); err != nil || !active {
		t.Fatalf("inactive route: %v", err)
	}
	hit := map[string]string{"id": "99999999560", "user_id": "sync-560-1", "type": "profile", "date": "2024-01-01 12:00:00"}
	for i := range 2 {
		done, err := repo.syncHit(ctx, hit, false)
		if err != nil || done != (i == 0) {
			t.Fatalf("hit %d done=%v err=%v", i, done, err)
		}
	}
}

func TestTemplateAndBackgroundMapping(t *testing.T) {
	for template, want := range map[string]string{"one": "classic_light", "two": "compact_card", "three": "classic_light", "unknown": "classic_light"} {
		got, err := Transform(LegacyUser{"user_id": "1", "uniqid_id": "abcde", "template": template, "background_type": "color", "background_value": "#123abc"}, nil)
		if err != nil || got.Theme != want || !bytes.Contains(got.Background, []byte("#123abc")) {
			t.Fatalf("template %s %+v %v", template, got, err)
		}
	}
	got, err := Transform(LegacyUser{"user_id": "1", "uniqid_id": "abcde", "background_type": "color", "background_value": "red;url(evil)"}, nil)
	if err != nil || len(got.Background) != 0 {
		t.Fatal("unsafe background accepted")
	}
}

func TestDefaultAvatarIsNotMedia(t *testing.T) {
	got, err := Transform(LegacyUser{"user_id": "1", "uniqid_id": "abcde", "avatar": "default.jpg"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got.Media {
		if m.Field == "avatar" {
			t.Fatalf("placeholder avatar referenced as media: %+v", m)
		}
	}
	got, err = Transform(LegacyUser{"user_id": "1", "uniqid_id": "abcde", "avatar": "9fb0ef66357ed7603c61bf3dff03842f.jpg"}, nil)
	if err != nil || len(got.Media) != 1 || got.Media[0].Path != "uploads/avatars/9fb0ef66357ed7603c61bf3dff03842f.jpg" {
		t.Fatalf("real avatar not mapped: %+v %v", got.Media, err)
	}
}

func TestSyncUsernameConflict(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	tr, err := Transform(LegacyUser{"user_id": "sync-conflict", "uniqid_id": "syncconflicthash", "username": "orders", "name": "Owner"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	media := &MediaSync{DryRun: true}
	if err = media.Load(ctx); err != nil {
		t.Fatal(err)
	}
	opts := options()
	opts.DryRun = true
	res, err := NewPostgres(d.Pool).syncUser(ctx, &tr, opts, syncRules{}, media, map[string]string{})
	if err != nil || res.Username != "conflict" || res.Status != "created" {
		t.Fatalf("conflict %+v err=%v", res, err)
	}
}

func TestManifestSkipsUnsupportedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"path":"template/images/favicon.ico","size":1,"mtime":1,"sha1":"caed0b8b3df72dedaf10a8b0987dead8ead3715d"},`+
			`{"path":"uploads/avatars/a.jpg","size":1,"mtime":1,"sha1":"caed0b8b3df72dedaf10a8b0987dead8ead3715d"}]`)
	}))
	defer server.Close()
	m := &MediaSync{Client: &ExportClient{URL: server.URL, Token: "secret"}}
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(m.Manifest) != 1 || m.Manifest["uploads/avatars/a.jpg"].Path == "" {
		t.Fatalf("manifest = %v", m.Manifest)
	}
}

func TestRemoteMediaRetriesDroppedConnection(t *testing.T) {
	body := "same bytes"
	sum := "4f1a9d5d4c1f6c3e5d2a2ab5b2d8b6a1a6c2a3f0"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mode") == "media_manifest" {
			_, _ = io.WriteString(w, `[{"path":"uploads/avatars/a.png","size":10,"mtime":1,"sha1":"`+sum+`"}]`)
			return
		}
		calls++
		if calls == 1 {
			// Simulate the host resetting the connection mid-body.
			w.Header().Set("Content-Length", "10")
			_, _ = io.WriteString(w, "same")
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	m := &MediaSync{Client: &ExportClient{URL: server.URL, Token: "secret"}, Driver: storage.NewMemory()}
	ctx := context.Background()
	if err := m.Load(ctx); err != nil {
		t.Fatal(err)
	}
	m.Manifest["uploads/avatars/a.png"] = MediaEntry{Path: "uploads/avatars/a.png", Size: 10, SHA1: sha1Hex(body)}
	if _, err := m.Put(ctx, "profile", MediaRef{Field: "avatar", Path: "uploads/avatars/a.png"}); err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func sha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
