package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/registry"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func TestDefinitionDuplicateKeyReturnsConflict(t *testing.T) {
	database := pgtest.Open(t)
	h := New(nil, registry.NewCachedStore(database.Queries, time.Minute))
	body := fmt.Sprintf(`{"key":%q,"name":"Test","category":"test","schema_version":1,"retention_days":30,"required_properties":[]}`, "test."+uuid.NewString())
	for _, status := range []int{http.StatusCreated, http.StatusConflict} {
		w := httptest.NewRecorder()
		h.CreateDefinition(w, httptest.NewRequest(http.MethodPost, "/v1/platform/events/definitions", strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body.String())
		}
		if status == http.StatusConflict {
			var envelope struct {
				Success bool `json:"success"`
				Error   struct {
					Code string `json:"code"`
				} `json:"error"`
				Meta map[string]any `json:"meta"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Success || envelope.Error.Code != "CONFLICT" || envelope.Meta == nil {
				t.Fatalf("invalid conflict envelope: %s", w.Body.String())
			}
		}
	}
}

func TestDefinitionPatchPreservesAndClearsFields(t *testing.T) {
	database := pgtest.Open(t)
	store := registry.NewCachedStore(database.Queries, time.Minute)
	h := New(nil, store)
	for _, tc := range []struct {
		name        string
		body        string
		description string
		enabled     bool
		properties  int
	}{
		{"absent", `{}`, "original", true, 1},
		{"unrelated", `{"name":"Renamed"}`, "original", true, 1},
		{"empty", `{"description":""}`, "", true, 1},
		{"null", `{"description":null}`, "", true, 1},
		{"replace", `{"description":"replacement"}`, "replacement", true, 1},
		{"false and empty list", `{"enabled":false,"is_public":false,"aggregate_enabled":false,"required_properties":[]}`, "original", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def, err := store.Create(t.Context(), domain.EventDefinition{
				Key: "test." + uuid.NewString(), Name: "Test", Description: "original", Category: "test",
				SchemaVersion: 1, RetentionDays: 30, Enabled: true, IsPublic: true, AggregateEnabled: true,
				RequiredProperties: []string{"source"},
			})
			if err != nil {
				t.Fatal(err)
			}
			path := fmt.Sprintf("/v1/platform/events/definitions/%d", def.ID)
			r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(tc.body))
			r.SetPathValue("id", fmt.Sprint(def.ID))
			w := httptest.NewRecorder()
			h.PatchDefinition(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var envelope struct {
				Data domain.EventDefinition `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			fresh, err := registry.NewCachedStore(database.Queries, time.Minute).GetByID(t.Context(), def.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range []domain.EventDefinition{envelope.Data, *fresh} {
				if got.Description != tc.description || got.Enabled != tc.enabled || got.IsPublic != tc.enabled ||
					got.AggregateEnabled != tc.enabled || len(got.RequiredProperties) != tc.properties ||
					got.Category != "test" || got.SchemaVersion != 1 || got.RetentionDays != 30 || got.Key != def.Key {
					t.Fatalf("unexpected definition: %+v", got)
				}
			}
		})
	}
}
