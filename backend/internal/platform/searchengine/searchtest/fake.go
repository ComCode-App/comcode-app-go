// Package searchtest is an in-memory Meilisearch stand-in for tests (TEC-951). It serves the
// endpoints searchengine.Client uses (index create/settings, document upsert, tasks, multi-search)
// and evaluates the one filter the client sends: "account_id = N".
package searchtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
)

// Query is one received multi-search query.
type Query struct {
	IndexUID string `json:"indexUid"`
	Q        string `json:"q"`
	Limit    int    `json:"limit"`
	Filter   any    `json:"filter"`
}

// Meili is the fake server.
type Meili struct {
	URL string
	// IgnoreFilter makes searches skip filters, simulating a misconfigured index.
	IgnoreFilter bool

	mu       sync.Mutex
	docs     map[string]map[string]map[string]any // index -> id -> document
	settings map[string]map[string]any
	queries  []Query
	task     int
}

// New starts a fake server closed with the test.
func New(t testing.TB) *Meili {
	t.Helper()
	m := &Meili{docs: map[string]map[string]map[string]any{}, settings: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(srv.Close)
	m.URL = srv.URL
	return m
}

// Config returns a search config pointing at the fake server.
func (m *Meili) Config() config.SearchConfig {
	return config.SearchConfig{Enabled: true, Driver: "meilisearch", MeiliHost: m.URL, MeiliKey: "test", IndexPrefix: "test"}
}

// Queries returns the multi-search queries received so far.
func (m *Meili) Queries() []Query {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Query(nil), m.queries...)
}

// Settings returns the last settings stored for an index.
func (m *Meili) Settings(index string) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings[index]
}

func (m *Meili) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/indexes":
		m.enqueued(w)
	case r.Method == http.MethodPatch && len(parts) == 3 && parts[0] == "indexes" && parts[2] == "settings":
		var body map[string]any
		if !decode(w, r, &body) {
			return
		}
		m.settings[parts[1]] = body
		m.enqueued(w)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "indexes" && parts[2] == "documents":
		var docs []map[string]any
		if !decode(w, r, &docs) {
			return
		}
		if m.docs[parts[1]] == nil {
			m.docs[parts[1]] = map[string]map[string]any{}
		}
		for _, d := range docs {
			m.docs[parts[1]][fmt.Sprint(d["id"])] = d
		}
		m.enqueued(w)
	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "tasks":
		writeJSON(w, http.StatusOK, map[string]any{"uid": json.Number(parts[1]), "status": "succeeded"})
	case r.Method == http.MethodPost && r.URL.Path == "/multi-search":
		var body struct {
			Queries []Query `json:"queries"`
		}
		if !decode(w, r, &body) {
			return
		}
		results := make([]map[string]any, 0, len(body.Queries))
		for _, q := range body.Queries {
			m.queries = append(m.queries, q)
			hits, err := m.search(q)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error(), "code": "invalid_search_filter"})
				return
			}
			results = append(results, map[string]any{"indexUid": q.IndexUID, "hits": hits, "query": q.Q})
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "searchtest: unhandled " + r.Method + " " + r.URL.Path})
	}
}

func (m *Meili) search(q Query) ([]map[string]any, error) {
	var account string
	if q.Filter != nil && !m.IgnoreFilter {
		f, ok := q.Filter.(string)
		if !ok || !strings.HasPrefix(f, "account_id = ") {
			return nil, fmt.Errorf("searchtest: unsupported filter %v", q.Filter)
		}
		account = strings.TrimPrefix(f, "account_id = ")
	}
	needle := strings.ToLower(q.Q)
	hits := []map[string]any{}
	for _, d := range m.docs[q.IndexUID] {
		if account != "" && fmt.Sprint(d["account_id"]) != account {
			continue
		}
		if strings.Contains(strings.ToLower(fmt.Sprint(d["title"], " ", d["subtitle"], " ", d["keywords"])), needle) {
			hits = append(hits, d)
		}
	}
	return hits, nil
}

func (m *Meili) enqueued(w http.ResponseWriter) {
	m.task++
	writeJSON(w, http.StatusAccepted, map[string]any{"taskUid": m.task, "status": "enqueued"})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
