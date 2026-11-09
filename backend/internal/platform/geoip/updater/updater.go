package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	geoip2 "github.com/oschwald/geoip2-golang"
)

// UpdateOptions configures the GeoIP database updater.
type UpdateOptions struct {
	DestinationPath string
	AccountID       string
	LicenseKey      string
	CustomURL       string
	HTTPClient      *http.Client
}

// Result describes the outcome of an update cycle.
type Result struct {
	Success      bool
	Version      string
	DownloadedAt time.Time
	Bytes        int64
	Error        error
}

// ResolveDestinationPath ensures the target directory is writable, falling back from /data/ to ./data/ on local macOS/non-root setups.
func ResolveDestinationPath(destPath string) string {
	if destPath == "" {
		return "data/geoip/GeoLite2-City.mmdb"
	}
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		if strings.HasPrefix(destPath, "/data/") {
			relPath := filepath.Join("data", strings.TrimPrefix(destPath, "/data/"))
			if mkErr := os.MkdirAll(filepath.Dir(relPath), 0755); mkErr == nil {
				return relPath
			}
		}
	}
	return destPath
}

// DownloadAndVerify fetches the latest GeoIP database, validates it, and performs an atomic replace.
func DownloadAndVerify(ctx context.Context, opts UpdateOptions) (*Result, error) {
	destPath := ResolveDestinationPath(opts.DestinationPath)
	if destPath == "" {
		return nil, errors.New("destination path is required")
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}

	var resp *http.Response
	var finalURL string
	var err error

	if opts.CustomURL != "" {
		finalURL = opts.CustomURL
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
		if reqErr != nil {
			return nil, fmt.Errorf("create custom download request: %w", reqErr)
		}
		if opts.AccountID != "" && opts.LicenseKey != "" {
			req.SetBasicAuth(opts.AccountID, opts.LicenseKey)
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("download from custom url: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("download returned non-200 status: %s", resp.Status)
		}
	} else {
		if opts.LicenseKey == "" {
			return nil, errors.New("MaxMind License Key (and Account ID) or custom download URL is required")
		}

		// 1. Try modern MaxMind REST download endpoint with HTTP Basic Auth (AccountID:LicenseKey)
		if opts.AccountID != "" {
			restURL := "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz"
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, restURL, nil)
			if reqErr == nil {
				req.SetBasicAuth(opts.AccountID, opts.LicenseKey)
				r, doErr := client.Do(req)
				if doErr == nil && r.StatusCode == http.StatusOK {
					resp = r
					finalURL = restURL
				} else if r != nil {
					_ = r.Body.Close()
				}
			}
		}

		// 2. Fallback to legacy query parameter endpoint
		if resp == nil {
			legacyURL := fmt.Sprintf(
				"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=%s&suffix=tar.gz",
				opts.LicenseKey,
			)
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, legacyURL, nil)
			if reqErr != nil {
				return nil, fmt.Errorf("create download request: %w", reqErr)
			}
			r, doErr := client.Do(req)
			if doErr != nil {
				return nil, fmt.Errorf("download geoip db: %w", doErr)
			}
			if r.StatusCode != http.StatusOK {
				_ = r.Body.Close()
				return nil, fmt.Errorf("download returned non-200 status: %s", r.Status)
			}
			resp = r
			finalURL = legacyURL
		}
	}
	defer func() { _ = resp.Body.Close() }()

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create destination directory %q: %w", destDir, err)
	}

	tmpFile, err := os.CreateTemp(destDir, "geoip_download_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	// Detect if response is tar.gz or raw mmdb
	contentType := resp.Header.Get("Content-Type")
	isTarGz := strings.Contains(contentType, "gzip") ||
		strings.Contains(contentType, "tar") ||
		strings.Contains(finalURL, "tar.gz")

	var bytesWritten int64
	if isTarGz {
		bytesWritten, err = extractMMDBFromTarGz(resp.Body, tmpFile)
		if err != nil {
			return nil, fmt.Errorf("extract mmdb archive: %w", err)
		}
	} else {
		bytesWritten, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return nil, fmt.Errorf("copy body to temp file: %w", err)
		}
	}
	_ = tmpFile.Close()

	// Validate the downloaded temporary MMDB before replacing
	reader, err := geoip2.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("validate downloaded mmdb: corrupt or invalid format: %w", err)
	}

	// Probe lookup sanity test
	testIP := net.ParseIP("8.8.8.8")
	if _, err := reader.City(testIP); err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("sanity check probe failed: %w", err)
	}

	version := "Unknown"
	if meta := reader.Metadata(); meta.BuildEpoch > 0 {
		version = time.Unix(int64(meta.BuildEpoch), 0).UTC().Format("2006-01-02 15:04:05 UTC")
	}
	_ = reader.Close()

	// Atomic rename/move to destination path
	if err := os.Rename(tmpPath, destPath); err != nil {
		if copyErr := copyFile(tmpPath, destPath); copyErr != nil {
			return nil, fmt.Errorf("atomic replace destination file: %w", copyErr)
		}
	}

	return &Result{
		Success:      true,
		Version:      version,
		DownloadedAt: time.Now().UTC(),
		Bytes:        bytesWritten,
	}, nil
}

func extractMMDBFromTarGz(r io.Reader, dst io.Writer) (int64, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return 0, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("tar next: %w", err)
		}

		if header.Typeflag == tar.TypeReg && strings.HasSuffix(header.Name, ".mmdb") {
			return io.Copy(dst, tr)
		}
	}

	return 0, errors.New("no .mmdb file found in tar.gz archive")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
