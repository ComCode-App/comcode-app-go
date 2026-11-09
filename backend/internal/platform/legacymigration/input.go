// Package legacymigration imports normalized card identities and repeatedly syncs
// verified v1 user/media exports into account-owned profiles.
package legacymigration

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Row is the version-one normalized input contract. CSV extra_json is a JSON object encoded
// in a CSV cell; JSONL extra_json is an object. created_at is optional RFC3339.
type Row struct {
	LegacyHash  string          `json:"legacy_hash"`
	TargetType  string          `json:"target_type"`
	TargetValue string          `json:"target_value"`
	OwnerEmail  string          `json:"owner_email"`
	OwnerName   string          `json:"owner_name"`
	CreatedAt   string          `json:"created_at"`
	ExtraJSON   json.RawMessage `json:"extra_json"`
}

var columns = []string{"legacy_hash", "target_type", "target_value", "owner_email", "owner_name", "created_at", "extra_json"}
var hashPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,118}$`)

// Key follows the existing case-insensitive resolver, without changing the source hash.
func (r Row) Key() string { return strings.ToLower(r.LegacyHash) }

func (r *Row) validate() error {
	if !hashPattern.MatchString(r.LegacyHash) || r.LegacyHash == "." || r.LegacyHash == ".." {
		return errors.New("legacy_hash must be a single ASCII URL segment, 1–119 characters")
	}
	switch r.TargetType {
	case "profile", "url", "vcard", "smart_link":
		if r.TargetValue == "" {
			return errors.New("target_value is required")
		}
	case "unknown":
		if r.TargetValue != "" {
			return errors.New("unknown target must have an empty target_value")
		}
	default:
		return errors.New("invalid target_type")
	}
	if r.CreatedAt != "" {
		if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
			return fmt.Errorf("created_at: %w", err)
		}
	}
	if len(r.ExtraJSON) == 0 {
		r.ExtraJSON = json.RawMessage(`{}`)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(r.ExtraJSON, &obj); err != nil || obj == nil {
		return errors.New("extra_json must be a JSON object")
	}
	// Canonical object encoding makes CSV and JSONL reruns equivalent.
	var err error
	r.ExtraJSON, err = json.Marshal(obj)
	return err
}

// Reader streams either CSV or JSONL; it never loads the whole input into memory.
type Reader struct {
	csv   *csv.Reader
	lines *bufio.Scanner
	line  int
}

func NewReader(in io.Reader, format string) (*Reader, error) {
	r := &Reader{}
	switch format {
	case "csv":
		r.csv = csv.NewReader(in)
		header, err := r.csv.Read()
		if err != nil {
			return nil, fmt.Errorf("CSV header: %w", err)
		}
		if len(header) != len(columns) {
			return nil, errors.New("CSV requires the seven normalized columns in documented order")
		}
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
		for i, c := range columns {
			if header[i] != c {
				return nil, fmt.Errorf("CSV column %d must be %s", i+1, c)
			}
		}
		r.line = 1
	case "jsonl":
		r.lines = bufio.NewScanner(in)
		r.lines.Buffer(make([]byte, 65536), 4*1024*1024)
	default:
		return nil, errors.New("format must be csv or jsonl")
	}
	return r, nil
}

func (r *Reader) Next() (Row, error) {
	var row Row
	r.line++
	if r.csv != nil {
		values, err := r.csv.Read()
		if err != nil {
			return row, err
		}
		row = Row{values[0], values[1], values[2], values[3], values[4], values[5], json.RawMessage(values[6])}
	} else {
		if !r.lines.Scan() {
			if err := r.lines.Err(); err != nil {
				return row, fmt.Errorf("JSONL read: %w", err)
			}
			return row, io.EOF
		}
		dec := json.NewDecoder(strings.NewReader(r.lines.Text()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&row); err != nil {
			return row, fmt.Errorf("row %d: %w", r.line, err)
		}
		if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
			return row, fmt.Errorf("row %d: trailing JSON", r.line)
		}
	}
	if err := row.validate(); err != nil {
		return row, fmt.Errorf("row %d: %w", r.line, err)
	}
	return row, nil
}
