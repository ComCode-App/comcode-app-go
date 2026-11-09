package legacymigration

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
)

// Options controls privileged, offline migration operations; no app scope is bypassed by HTTP.
type Options struct {
	BatchID     string
	BatchSize   int
	DryRun      bool
	Account     string // holding (default) or an existing active account UUID
	MatchOwners bool
	FrontendURL string
}

const BlockedAppPath = "blocked: collides with app path"

type Result struct{ Hash, Status, Warning string }
type Summary struct{ Rows, Created, Skipped, Pending, Blocked int }

type Repository interface {
	Batch(context.Context, []Row, Options) ([]Result, error)
	Expected(context.Context, Row, Options) (string, error)
}
type Resolver interface {
	Resolve(context.Context, string, ...ingestion.HTTPContext) (*model.RouteResolution, error)
}

// Import processes bounded transactions; an error rolls back only the current batch.
func Import(ctx context.Context, repo Repository, reader *Reader, opts Options, report io.Writer, samples io.Writer) (Summary, error) {
	var summary Summary
	if opts.BatchSize < 1 || opts.BatchSize > 10000 {
		return summary, errors.New("batch-size must be 1–10000")
	}
	if opts.BatchID == "" {
		return summary, errors.New("batch-id is required")
	}
	w := csv.NewWriter(report)
	if err := w.Write([]string{"hash", "status", "warning"}); err != nil {
		return summary, err
	}
	defer w.Flush()
	for {
		batch := make([]Row, 0, opts.BatchSize)
		for len(batch) < opts.BatchSize {
			row, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return summary, err
			}
			if summary.Rows+len(batch) < 3 && samples != nil {
				if _, err := fmt.Fprintf(samples, "sample: hash=%q type=%q target=%q\n", row.LegacyHash, row.TargetType, row.TargetValue); err != nil {
					return summary, err
				}
			}
			batch = append(batch, row)
		}
		if len(batch) == 0 {
			break
		}
		results, err := repo.Batch(ctx, batch, opts)
		if err != nil {
			return summary, fmt.Errorf("batch after %d committed rows: %w", summary.Rows, err)
		}
		for _, res := range results {
			summary.Rows++
			switch res.Status {
			case "created", "would_create":
				summary.Created++
			case "skipped":
				summary.Skipped++
			case BlockedAppPath:
				summary.Blocked++
			}
			if res.Warning != "" {
				summary.Pending++
			}
			if err := w.Write([]string{res.Hash, res.Status, res.Warning}); err != nil {
				return summary, err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return summary, err
		}
	}
	// A named summary section makes blocked rows visible even in an empty report.
	if err := w.Write([]string{BlockedAppPath, "count", fmt.Sprint(summary.Blocked)}); err != nil {
		return summary, err
	}
	w.Flush()
	return summary, w.Error()
}
