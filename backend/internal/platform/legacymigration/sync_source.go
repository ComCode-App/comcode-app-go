package legacymigration

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha1" // Legacy manifest compatibility; not used for authentication.
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

type ExportClient struct {
	URL, Token string
	HTTP       *http.Client
}

// Open suppresses transport errors containing token-bearing URLs and refuses redirects.
func (c *ExportClient) Open(ctx context.Context, mode string, params url.Values) (io.ReadCloser, error) {
	u, err := url.Parse(c.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("invalid exporter URL")
	}
	q := u.Query()
	q.Set("mode", mode)
	q.Set("token", c.Token)
	for k, v := range params {
		q[k] = v
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("cannot create exporter request")
	}
	client := http.Client{Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if c.HTTP != nil {
		client = *c.HTTP
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("exporter request failed (URL and credentials suppressed)")
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("exporter returned HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

type csvStream struct {
	*csv.Reader
	closers []io.Closer
}

func openCSV(in io.ReadCloser) (*csvStream, error) {
	b := bufio.NewReader(in)
	r := io.Reader(b)
	closers := []io.Closer{in}
	magic, _ := b.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		z, err := gzip.NewReader(b)
		if err != nil {
			_ = in.Close()
			return nil, err
		}
		r = z
		closers = append(closers, z)
	}
	return &csvStream{Reader: csv.NewReader(r), closers: closers}, nil
}
func (c *csvStream) Close() error {
	var err error
	for _, cl := range c.closers {
		err = errors.Join(err, cl.Close())
	}
	return err
}
func readCSV(ctx context.Context, c *ExportClient, file, table, since string, fn func(map[string]string) error) error {
	var in io.ReadCloser
	var err error
	if file != "" {
		in, err = os.Open(file)
	} else {
		mode := "dump"
		params := url.Values{"table": {table}}
		if table == "hits" {
			mode = "hits_since"
			params = url.Values{"since": {since}}
		}
		in, err = c.Open(ctx, mode, params)
	}
	if err != nil {
		return err
	}
	stream, err := openCSV(in)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()
	header, err := stream.Read()
	if err != nil {
		return err
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	seen := map[string]bool{}
	for _, h := range header {
		if seen[h] {
			return errors.New("duplicate CSV column")
		}
		seen[h] = true
	}
	for {
		values, err := stream.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		row := map[string]string{}
		for i, k := range header {
			row[k] = values[i]
		}
		if err := fn(row); err != nil {
			return err
		}
	}
}

type MediaEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
	SHA1  string `json:"sha1"`
}
type MediaSnapshot map[string]MediaEntry
type MediaUpload struct{ SHA1, Key, URL string }
type MediaDriver interface {
	Upload(context.Context, storage.File, string) error
	Exists(context.Context, string) (bool, error)
	PublicURL(string) string
}

type MediaSync struct {
	Dir            string
	Client         *ExportClient
	Driver         MediaDriver
	Manifest       MediaSnapshot
	Uploads        map[string]MediaUpload
	DryRun         bool
	Planned        map[string]bool
	Count, Missing int
}

func validMediaPath(p string) bool {
	if strings.Contains(p, "\\") || strings.Contains(p, "\x00") || path.Clean(p) != p || strings.HasPrefix(p, "/") {
		return false
	}
	allowed := false
	for _, root := range []string{"uploads/avatars/", "uploads/comps/", "uploads/kataloglar/", "usercard/", "template/images/"} {
		if strings.HasPrefix(p, root) {
			allowed = true
		}
	}
	ext := strings.ToLower(path.Ext(p))
	return allowed && (imageName(p) || ext == ".pdf")
}
func localMedia(dir, rel string) (string, error) {
	if !validMediaPath(rel) {
		return "", errors.New("invalid media path")
	}
	base, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	for _, prefix := range []string{"", "public_html"} {
		candidate := filepath.Join(base, prefix, filepath.FromSlash(rel))
		real, err := filepath.EvalSymlinks(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		// Reject symlinks even when they point inside the root.
		if real != candidate {
			return "", errors.New("symlink media refused")
		}
		return candidate, nil
	}
	return "", os.ErrNotExist
}
func (m *MediaSync) Load(ctx context.Context) error {
	m.Manifest = MediaSnapshot{}
	m.Planned = map[string]bool{}
	if m.Uploads == nil {
		m.Uploads = map[string]MediaUpload{}
	}
	if m.Dir != "" {
		return nil
	}
	if m.Client == nil || m.Client.URL == "" {
		return nil
	}
	in, err := m.Client.Open(ctx, "media_manifest", nil)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	var entries []MediaEntry
	if err := json.NewDecoder(in).Decode(&entries); err != nil {
		return err
	}
	for _, e := range entries {
		// The PHP side lists every file type it serves (e.g. favicon.ico); keep only media the sync can import.
		if !validMediaPath(e.Path) {
			continue
		}
		if len(e.SHA1) != 40 {
			return errors.New("invalid manifest entry")
		}
		if _, err := hex.DecodeString(e.SHA1); err != nil {
			return err
		}
		m.Manifest[e.Path] = e
	}
	return nil
}
func (m *MediaSync) Put(ctx context.Context, profile string, ref MediaRef) (string, error) {
	var in io.ReadCloser
	var e MediaEntry
	var err error
	if m.Dir != "" {
		file, err := localMedia(m.Dir, ref.Path)
		if errors.Is(err, os.ErrNotExist) {
			m.Missing++
			return "", nil
		}
		if err != nil {
			return "", err
		}
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return "", err
		}
		h := sha1.New()
		if _, err = io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", err
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return "", err
		}
		e = MediaEntry{Path: ref.Path, Size: info.Size(), MTime: info.ModTime().Unix(), SHA1: hex.EncodeToString(h.Sum(nil))}
		m.Manifest[ref.Path] = e
		in = f
	} else {
		var ok bool
		e, ok = m.Manifest[ref.Path]
		if !ok {
			m.Missing++
			return "", nil
		}
	}
	if in != nil {
		defer func() { _ = in.Close() }()
	}
	// Dedupe is per profile to keep profile-owned media inside its own storage prefix.
	identity := profile + ":" + e.SHA1
	if old, ok := m.Uploads[identity]; ok {
		return old.URL, nil
	}
	key := storage.ProfileCompanyMediaObjectKey(profile, "legacy", e.SHA1, strings.TrimPrefix(path.Ext(ref.Path), "."))
	if !m.Planned[identity] {
		m.Count++
		m.Planned[identity] = true
	}
	if m.DryRun {
		return "", nil
	}
	if m.Driver == nil {
		return "", errors.New("storage driver required for referenced media")
	}
	exists, err := m.Driver.Exists(ctx, key)
	if err != nil {
		return "", err
	}
	if !exists {
		// Verify remote bytes before upload; stage on disk to keep memory bounded.
		tmp, err := os.CreateTemp("", "legacy-media-*")
		if err != nil {
			return "", err
		}
		defer func() { _ = os.Remove(tmp.Name()); _ = tmp.Close() }()
		if in != nil {
			err = stageMedia(tmp, in, e)
		} else {
			// The shared cPanel host drops connections under load; retry the single file.
			for attempt := 0; attempt < 4; attempt++ {
				if attempt > 0 {
					select {
					case <-ctx.Done():
						return "", ctx.Err()
					case <-time.After(time.Duration(attempt) * 2 * time.Second):
					}
				}
				var body io.ReadCloser
				body, err = m.Client.Open(ctx, "media_file", url.Values{"path": {ref.Path}})
				if err == nil {
					err = stageMedia(tmp, body, e)
					_ = body.Close()
				}
				if err == nil {
					break
				}
			}
		}
		if err != nil {
			return "", err
		}
		contentType := mime.TypeByExtension(strings.ToLower(path.Ext(ref.Path)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if err = m.Driver.Upload(ctx, storage.File{Body: tmp, Size: e.Size, ContentType: contentType, Filename: path.Base(ref.Path)}, key); err != nil {
			return "", err
		}
	}
	result := MediaUpload{SHA1: e.SHA1, Key: key, URL: m.Driver.PublicURL(key)}
	m.Uploads[identity] = result
	return result.URL, nil
}

// stageMedia copies one verified file into tmp, rewinding it so the caller can upload or retry.
func stageMedia(tmp *os.File, in io.Reader, e MediaEntry) error {
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := tmp.Truncate(0); err != nil {
		return err
	}
	h := sha1.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(in, e.Size+1))
	if err != nil {
		return err
	}
	if n != e.Size || hex.EncodeToString(h.Sum(nil)) != e.SHA1 {
		return errors.New("media changed during export; retry sync")
	}
	_, err = tmp.Seek(0, io.SeekStart)
	return err
}
