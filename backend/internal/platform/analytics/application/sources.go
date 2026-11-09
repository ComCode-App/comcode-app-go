package application

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
)

// SourceOrder is the fixed order of the source breakdown.
var SourceOrder = []string{domain.SourceQR, domain.SourceNFC, domain.SourceLink, domain.SourceLegacy}

// SourceCount is one entry source of the breakdown.
type SourceCount struct {
	Source     string  `json:"source"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// SubjectSources is the GET /v1/analytics/sources payload (TEC-953).
type SubjectSources struct {
	// Views counts page openings only: profile page views (card.view, imported legacy page hits)
	// for a profile, route scans that were not QR-opened for a card or link. QR scans are never
	// added to it.
	Views int64 `json:"views"`
	// QRScans counts branded QR scans (card.qr.scan) of the subject and its related subjects.
	QRScans int64 `json:"qr_scans"`
	// Sources splits the entries (scans) by source, always qr, nfc, link, legacy in this order.
	Sources []SourceCount `json:"sources"`
	// TotalEntries is the sum of Sources.
	TotalEntries int64 `json:"total_entries"`
}

// GetSources returns the views, QR scans and entry source breakdown of a subject, including the
// subjects that lead to it (a profile's card, smart link and legacy hash routes). Aggregates only.
func (s *AnalyticsService) GetSources(ctx context.Context, filter domain.AnalyticsFilter) (*SubjectSources, error) {
	if err := validateDateRange(&filter); err != nil {
		return nil, err
	}
	rows, err := s.repo.GetSourceCounts(ctx, filter.SubjectType, filter.SubjectID, filter.DateFrom, filter.DateTo)
	if err != nil {
		return nil, err
	}
	return BuildSubjectSources(filter.SubjectType, rows), nil
}

// BuildSubjectSources folds the per-source rollup rows into the dashboard numbers.
func BuildSubjectSources(subjectType string, rows []domain.SourceRow) *SubjectSources {
	bySource := make(map[string]int64, len(SourceOrder))
	out := &SubjectSources{}
	for _, r := range rows {
		switch r.EventType {
		case domain.EventQRScan:
			out.QRScans += r.Count
			bySource[domain.SourceQR] += r.Count
		case domain.EventRouteScan:
			// A QR-opened route scan is already counted by its card.qr.scan.
			if r.Source != domain.SourceQR && r.Source != "" {
				bySource[r.Source] += r.Count
			}
			if subjectType != "profile" && r.Source != domain.SourceQR {
				out.Views += r.Count
			}
		case domain.EventLegacyProfileView:
			bySource[domain.SourceLegacy] += r.Count
			if r.SubjectType == "profile" {
				out.Views += r.Count
			}
		case "card.view":
			if subjectType == "profile" && r.SubjectType == "profile" {
				out.Views += r.Count
			}
		}
	}
	out.Sources = make([]SourceCount, 0, len(SourceOrder))
	for _, src := range SourceOrder {
		out.TotalEntries += bySource[src]
	}
	for _, src := range SourceOrder {
		c := SourceCount{Source: src, Count: bySource[src]}
		if out.TotalEntries > 0 {
			c.Percentage = float64(c.Count) / float64(out.TotalEntries) * 100
		}
		out.Sources = append(out.Sources, c)
	}
	return out
}
