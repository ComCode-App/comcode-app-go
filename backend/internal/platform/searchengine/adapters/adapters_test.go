package adapters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
)

// The Cmd+K role hit must open the role detail page, which lives under
// /platform/settings/roles/{uuid} (frontend/src/config/routes.ts).
func TestRolesAdapterHrefOpensSettingsRoleDetail(t *testing.T) {
	id := uuid.MustParse("6f1c2c1e-6a0e-4d5f-9f55-1c2b3d4e5f60")
	doc := (&RolesAdapter{}).documentFromRole(db.Role{Uuid: id, Name: "Editors", Slug: "editors"})

	want := "/platform/settings/roles/" + id.String()
	if doc.Href != want {
		t.Fatalf("Href = %q, want %q", doc.Href, want)
	}

	// The page that serves the link must exist in the Next.js app.
	page := filepath.Join("..", "..", "..", "..", "..", "frontend", "src", "app", "(shell)", "(platform)",
		"platform", "settings", "roles", "[uuid]", "page.tsx")
	if _, err := os.Stat(page); err != nil {
		t.Fatalf("role detail page for %s is missing: %v", want, err)
	}
}
