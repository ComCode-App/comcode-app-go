package legacymigration

import (
	"context"
	"path"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"github.com/jackc/pgx/v5"
)

// sectorIconFields are the legacy company image fields, the icon field first. Only ikon2
// holds a sector icon by design, so only its unmatched names are reported; the others match
// when they name a catalog icon (uploaded logos have hashed names and never do).
var sectorIconFields = []string{"ikon2", "arma1", "yer3", "logo4", "firmalogo"}

// legacySectorAliases are legacy icon names that differ from the TEC-571 catalog names.
// Catalog slugs and slugified Turkish/English names match without an entry here.
var legacySectorAliases = map[string]string{
	"bilisim":           "technology",
	"yazilim":           "technology",
	"danismanlik":       "professional-services",
	"nakliye":           "logistics",
	"saglik-hizmetleri": "healthcare",
}

// sectorIconKey turns "uploads/comps/İnşaat.PNG" into "insaat".
func sectorIconKey(name string) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	return publicpath.SlugifyName(strings.TrimSuffix(base, path.Ext(base)))
}

type sector struct {
	ID   int64
	Slug string
}

// sectorCatalog maps legacy icon keys to catalog sectors.
type sectorCatalog map[string]sector

func loadSectorCatalog(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) (sectorCatalog, error) {
	rows, err := q.Query(ctx, `SELECT id,slug,name_tr,name_en FROM sectors`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	catalog := sectorCatalog{}
	bySlug := map[string]sector{}
	for rows.Next() {
		var s sector
		var tr, en string
		if err = rows.Scan(&s.ID, &s.Slug, &tr, &en); err != nil {
			return nil, err
		}
		bySlug[s.Slug] = s
		for _, key := range []string{s.Slug, publicpath.SlugifyName(tr), publicpath.SlugifyName(en)} {
			if key != "" {
				catalog[key] = s
			}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for alias, slug := range legacySectorAliases {
		if s, ok := bySlug[slug]; ok {
			if _, taken := catalog[alias]; !taken {
				catalog[alias] = s
			}
		}
	}
	return catalog, nil
}

// match returns the sector of the first icon that names one.
func (c sectorCatalog) match(icons []string) (sector, bool) {
	for _, icon := range icons {
		if s, ok := c[sectorIconKey(icon)]; ok {
			return s, true
		}
	}
	return sector{}, false
}
