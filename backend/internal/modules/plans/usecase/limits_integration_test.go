package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func featureValueTypes(t *testing.T, q *db.Queries) map[string]string {
	t.Helper()
	rows, err := q.ListFeatureDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Key] = string(row.ValueType)
	}
	return out
}

// Every plan seeded by migrations (000022, 000039) must pass the limit validation, otherwise an
// admin could not re-save an untouched plan.
func TestSeededPlansPassLimitValidation(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	valueTypes := featureValueTypes(t, d.Queries)

	plans, err := d.Queries.ListPlansFiltered(ctx, db.ListPlansFilteredParams{LimitCount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	seeded := map[string]bool{}
	for _, plan := range plans {
		rows, err := d.Queries.ListPlanLimitsByPlanID(ctx, plan.ID)
		if err != nil {
			t.Fatal(err)
		}
		limits := make([]PlanLimitInput, 0, len(rows))
		for _, row := range rows {
			lim := PlanLimitInput{FeatureKey: row.FeatureKey}
			if row.ValueInt.Valid {
				v := int64(row.ValueInt.Int32)
				lim.IntValue = &v
			}
			if row.ValueBool.Valid {
				v := row.ValueBool.Bool
				lim.BoolValue = &v
			}
			limits = append(limits, lim)
		}
		if errs := ValidateLimits(valueTypes, limits); len(errs) > 0 {
			t.Errorf("plan %s: seeded limits fail validation: %+v", plan.Slug, errs)
		}
		seeded[plan.Slug] = true
	}
	for _, slug := range []string{"free", "pro-monthly", "pro-yearly"} {
		if !seeded[slug] {
			t.Errorf("seeded plan %s not found", slug)
		}
	}
}

// The code catalog and feature_definitions must hold the same features with the same value_type,
// since validation uses the DB.
func TestFeatureDefinitionsMatchCatalog(t *testing.T) {
	d := pgtest.Open(t)
	valueTypes := featureValueTypes(t, d.Queries)
	inCatalog := map[string]bool{}
	for _, f := range entitlement.Catalog() {
		inCatalog[f.Key] = true
		if got, ok := valueTypes[f.Key]; !ok || got != f.ValueType {
			t.Errorf("feature %s: db value_type %q (present %v), catalog %q", f.Key, got, ok, f.ValueType)
		}
	}
	for key := range valueTypes {
		if !inCatalog[key] {
			t.Errorf("feature %s is in feature_definitions but not in the code catalog", key)
		}
	}
}

// fullLimits returns a valid value for every catalog feature, with overrides by key.
func fullLimits(overrides map[string]int64) []PlanLimitInput {
	out := make([]PlanLimitInput, 0, len(entitlement.Catalog()))
	for _, f := range entitlement.Catalog() {
		lim := PlanLimitInput{FeatureKey: f.Key}
		if f.ValueType == "bool" {
			lim.BoolValue = bptr(true)
		} else {
			v, ok := overrides[f.Key]
			if !ok {
				v = 1
			}
			lim.IntValue = i64(v)
		}
		out = append(out, lim)
	}
	return out
}

func TestCreateAndPatchRejectInvalidLimits(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc := New(d.Pool, d.Queries)
	slug := "tec861-" + pgtest.Unique()

	bad := fullLimits(nil)
	bad[0].BoolValue = bptr(true)
	_, err := svc.Create(ctx, CreatePlanInput{
		Slug: slug, Name: "Bad", BillingInterval: "month", PriceMinor: 1000, IsActive: true, Limits: bad,
	})
	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Fields[0].Field != "limits."+bad[0].FeatureKey {
		t.Fatalf("create: want ValidationError on limits.%s, got %v", bad[0].FeatureKey, err)
	}
	if _, err := d.Queries.GetPlanBySlug(ctx, slug); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("invalid create must not insert the plan, got %v", err)
	}

	plan, err := svc.Create(ctx, CreatePlanInput{
		Slug: slug, Name: "Good", BillingInterval: "month", PriceMinor: 1000, IsActive: true,
		Limits: fullLimits(map[string]int64{
			entitlement.FeatureProfilesMax:   3,
			entitlement.FeatureSmartLinksMax: int64(entitlement.Unlimited),
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(plan.Limits) != len(entitlement.Catalog()) {
		t.Fatalf("want %d limits, got %+v", len(entitlement.Catalog()), plan.Limits)
	}

	unknown := append(fullLimits(nil), PlanLimitInput{FeatureKey: "unknown.feature", IntValue: i64(1)})
	if _, err := svc.Patch(ctx, plan.UUID, PatchPlanInput{Limits: &unknown}); !errors.As(err, &verr) ||
		verr.Fields[0].Code != LimitCodeUnknown {
		t.Fatalf("patch: want unknown_feature, got %v", err)
	}
	after, err := svc.Get(ctx, plan.UUID)
	if err != nil || len(after.Limits) != len(entitlement.Catalog()) {
		t.Fatalf("invalid patch must keep limits, got %+v (%v)", after.Limits, err)
	}

	// PATCH limits replaces the whole set.
	patched, err := svc.Patch(ctx, plan.UUID, PatchPlanInput{Limits: ptr(fullLimits(map[string]int64{entitlement.FeatureProfilesMax: 5}))})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	for _, l := range patched.Limits {
		if l.FeatureKey == entitlement.FeatureProfilesMax && *l.IntValue != 5 {
			t.Fatalf("patch must replace the limit set, got %+v", patched.Limits)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// TEC-942: a plan without a value for every feature is rejected on create and on PATCH.
func TestCreateAndPatchRequireEveryFeature(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc := New(d.Pool, d.Queries)
	slug := "tec942-" + pgtest.Unique()

	partial := fullLimits(nil)
	missing := partial[len(partial)-1].FeatureKey
	partial = partial[:len(partial)-1]
	_, err := svc.Create(ctx, CreatePlanInput{
		Slug: slug, Name: "Partial", BillingInterval: "month", PriceMinor: 1000, Limits: partial,
	})
	var verr *ValidationError
	if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != "limits."+missing ||
		verr.Fields[0].Code != LimitCodeRequired {
		t.Fatalf("create: want required error on limits.%s, got %v", missing, err)
	}
	if _, err := svc.Create(ctx, CreatePlanInput{Slug: slug, Name: "None", BillingInterval: "month", PriceMinor: 1000}); !errors.As(err, &verr) ||
		len(verr.Fields) != len(entitlement.Catalog()) {
		t.Fatalf("create without limits: want a required error per feature, got %v", err)
	}

	plan, err := svc.Create(ctx, CreatePlanInput{
		Slug: slug, Name: "Full", BillingInterval: "month", PriceMinor: 1000, Limits: fullLimits(nil),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Patch(ctx, plan.UUID, PatchPlanInput{Limits: &partial}); !errors.As(err, &verr) {
		t.Fatalf("patch: want ValidationError, got %v", err)
	}
	// Leaving limits out of a PATCH keeps the current set.
	if _, err := svc.Patch(ctx, plan.UUID, PatchPlanInput{Name: ptr("Renamed")}); err != nil {
		t.Fatalf("patch name: %v", err)
	}
}

// TEC-942: every plan in the database defines every catalog feature, so migrations that add a
// feature must write a value for the existing plans too.
func TestEveryPlanDefinesEveryFeature(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	plans, err := d.Queries.ListPlansFiltered(ctx, db.ListPlansFilteredParams{LimitCount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range plans {
		rows, err := d.Queries.ListPlanLimitsByPlanID(ctx, plan.ID)
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for _, row := range rows {
			have[row.FeatureKey] = row.ValueInt.Valid || row.ValueBool.Valid
		}
		for _, f := range entitlement.Catalog() {
			if !have[f.Key] {
				t.Errorf("plan %s has no value for %s", plan.Slug, f.Key)
			}
		}
	}
}

// TEC-942: a DB error while writing limits rolls back the plan row (create) and the plan
// update (patch).
func TestPlanWriteIsAtomic(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc := New(d.Pool, d.Queries)
	fn := "tec942_fail_" + pgtest.Unique()
	// Fails any limit insert of a plan whose name is "Fail limits".
	for _, stmt := range []string{
		`CREATE FUNCTION ` + fn + `() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (SELECT name FROM plans WHERE id = NEW.plan_id) = 'Fail limits' THEN
    RAISE EXCEPTION 'injected plan_limits failure';
  END IF;
  RETURN NEW;
END $$`,
		`CREATE TRIGGER ` + fn + ` BEFORE INSERT ON plan_limits FOR EACH ROW EXECUTE FUNCTION ` + fn + `()`,
	} {
		if _, err := d.Pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS `+fn+` ON plan_limits`)
		_, _ = d.Pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS `+fn+`()`)
	})

	slug := "tec942-" + pgtest.Unique()
	if _, err := svc.Create(ctx, CreatePlanInput{
		Slug: slug, Name: "Fail limits", BillingInterval: "month", PriceMinor: 1000, Limits: fullLimits(nil),
	}); err == nil {
		t.Fatal("create: want the injected limit error")
	}
	if _, err := d.Queries.GetPlanBySlug(ctx, slug); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("failed limit write must roll back the plan row, got %v", err)
	}

	plan, err := svc.Create(ctx, CreatePlanInput{
		Slug: slug, Name: "Atomic", BillingInterval: "month", PriceMinor: 1000,
		Limits: fullLimits(map[string]int64{entitlement.FeatureProfilesMax: 3}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Patch(ctx, plan.UUID, PatchPlanInput{
		Name: ptr("Fail limits"), PriceMinor: ptr(int64(2000)), Limits: ptr(fullLimits(map[string]int64{entitlement.FeatureProfilesMax: 7})),
	}); err == nil {
		t.Fatal("patch: want the injected limit error")
	}
	after, err := svc.Get(ctx, plan.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != "Atomic" || after.PriceMinor != 1000 || len(after.Limits) != len(entitlement.Catalog()) {
		t.Fatalf("failed patch must keep the plan and its limits, got %+v", after)
	}
	for _, l := range after.Limits {
		if l.FeatureKey == entitlement.FeatureProfilesMax && *l.IntValue != 3 {
			t.Fatalf("failed patch changed %s to %d", l.FeatureKey, *l.IntValue)
		}
	}
}
