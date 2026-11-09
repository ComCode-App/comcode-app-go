package publicpath

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	ok := []string{"furkan", "furkan-meclis", "f.meclis", "ali_1", "abc", "a1b"}
	bad := []string{"", "ab", "-abc", "abc-", "a--b", "a._b", "Furkan", "fur kan", "çağrı", "a/b",
		"login", "profiles", "account", "company", "_next", "platform", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	for _, u := range ok {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("%q: unexpected %v", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("%q: expected error", u)
		}
	}
}

func TestSlugifyName(t *testing.T) {
	cases := map[string][]string{
		"furkan-meclis":   {"Furkan", "Meçlis"},
		"cagri-ozturk":    {"ÇAĞRI", "Öztürk"},
		"ilker-isik":      {"İlker", "Işık"},
		"acme-teknoloji":  {"  Acme  Teknoloji!! "},
		"":                {"", ""},
		"jean-luc-picard": {"Jean-Luc", "Picard"},
	}
	for want, parts := range cases {
		if got := SlugifyName(parts...); got != want {
			t.Errorf("SlugifyName(%q) = %q, want %q", parts, got, want)
		}
	}
	long := SlugifyName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if len(long) > MaxUsername-4 {
		t.Errorf("slug too long: %d", len(long))
	}
}

// Scan source routes rather than a generated inventory, which could drift independently.
func TestFrontendTopLevelRoutesAreReserved(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "frontend", "src", "app")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("frontend source absent in this build context")
	} else if err != nil {
		t.Fatal(err)
	}
	segments := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), "_") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		file := parts[len(parts)-1]
		ext := filepath.Ext(file)
		stem := strings.TrimSuffix(file, ext)
		code := ext == ".tsx" || ext == ".ts" || ext == ".jsx" || ext == ".js"
		// Next metadata files serve paths even without page.tsx or route.ts.
		metadata := stem == "icon" || stem == "apple-icon" || stem == "favicon" ||
			stem == "robots" || stem == "sitemap" || stem == "manifest" ||
			stem == "opengraph-image" || stem == "twitter-image"
		route := (stem == "page" || stem == "route") && code
		if !route && !metadata {
			return nil
		}
		for _, part := range parts[:len(parts)-1] {
			if strings.HasPrefix(part, "(") && strings.HasSuffix(part, ")") {
				continue
			}
			if strings.HasPrefix(part, "@") {
				continue
			}
			if !strings.HasPrefix(part, "[") {
				segments[part] = true
			}
			return nil
		}
		if metadata {
			segment := file
			if code {
				segment = stem
				switch stem {
				case "robots":
					segment = "robots.txt"
				case "sitemap":
					segment = "sitemap.xml"
				case "manifest":
					segment = "manifest.webmanifest"
				}
			}
			segments[segment] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) == 0 {
		t.Fatal("no served frontend segments found")
	}
	for segment := range segments {
		if !IsReserved(segment) {
			t.Errorf("frontend serves /%s: add %q to reserved public paths", segment, segment)
		}
		// A static page would shadow the printed card that owns this reserved word.
		if IsLegacyBound(segment) {
			t.Errorf("frontend serves /%s, which a printed legacy card owns (TEC-955)", segment)
		}
	}
}

func TestLegacyBoundWordsStayReserved(t *testing.T) {
	for segment := range legacyBound {
		if !IsReserved(segment) {
			t.Errorf("legacy-bound %q must stay reserved", segment)
		}
		if ValidateUsername(segment) == nil {
			t.Errorf("legacy-bound %q must not be claimable as a username", segment)
		}
	}
	if !IsLegacyBound(" Comcode ") || IsLegacyBound("login") {
		t.Fatal("IsLegacyBound must match only the legacy exceptions, case-insensitively")
	}
}
