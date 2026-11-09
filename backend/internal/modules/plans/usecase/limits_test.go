package usecase

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func i64(v int64) *int64 { return &v }
func bptr(v bool) *bool  { return &v }

// The catalog has no bool feature today; "test.flag" keeps the bool rules covered.
var testValueTypes = map[string]string{
	"profiles.max": "int",
	"test.flag":    "bool",
}

// withOthers completes limits with a valid value for every test feature it does not list.
func withOthers(limits ...PlanLimitInput) []PlanLimitInput {
	listed := map[string]bool{}
	for _, l := range limits {
		listed[l.FeatureKey] = true
	}
	if !listed["profiles.max"] {
		limits = append(limits, PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(1)})
	}
	if !listed["test.flag"] {
		limits = append(limits, PlanLimitInput{FeatureKey: "test.flag", BoolValue: bptr(true)})
	}
	return limits
}

func TestValidateLimits_Valid(t *testing.T) {
	cases := [][]PlanLimitInput{
		withOthers(PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(0)}),
		withOthers(PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(-1)}),
		withOthers(PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(math.MaxInt32)}),
		{{FeatureKey: "test.flag", BoolValue: bptr(false)}, {FeatureKey: "profiles.max", IntValue: i64(10)}},
	}
	for i, limits := range cases {
		if got := ValidateLimits(testValueTypes, limits); len(got) != 0 {
			t.Errorf("case %d: unexpected errors %+v", i, got)
		}
	}
}

func TestValidateLimits_Invalid(t *testing.T) {
	cases := []struct {
		name  string
		limit PlanLimitInput
		field string
		code  string
	}{
		{"empty key", PlanLimitInput{IntValue: i64(1)}, "limits[0].feature_key", LimitCodeRequired},
		{"unknown key", PlanLimitInput{FeatureKey: "nope.max", IntValue: i64(1)}, "limits[0].feature_key", LimitCodeUnknown},
		{"bool on int feature", PlanLimitInput{FeatureKey: "profiles.max", BoolValue: bptr(true)}, "limits.profiles.max", LimitCodeType},
		{"int on bool feature", PlanLimitInput{FeatureKey: "test.flag", IntValue: i64(1)}, "limits.test.flag", LimitCodeType},
		{"both values", PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(1), BoolValue: bptr(true)}, "limits.profiles.max", LimitCodeType},
		{"no int value", PlanLimitInput{FeatureKey: "profiles.max"}, "limits.profiles.max", LimitCodeRequired},
		{"no bool value", PlanLimitInput{FeatureKey: "test.flag"}, "limits.test.flag", LimitCodeRequired},
		{"below unlimited", PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(-2)}, "limits.profiles.max", LimitCodeRange},
		{"above int32", PlanLimitInput{FeatureKey: "profiles.max", IntValue: i64(math.MaxInt32 + 1)}, "limits.profiles.max", LimitCodeRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateLimits(testValueTypes, withOthers(tc.limit))
			if len(got) != 1 || got[0].Field != tc.field || got[0].Code != tc.code || got[0].Message == "" {
				t.Fatalf("got %+v, want field %q code %q", got, tc.field, tc.code)
			}
		})
	}
}

func TestValidateLimits_DuplicateAndMultiple(t *testing.T) {
	got := ValidateLimits(testValueTypes, []PlanLimitInput{
		{FeatureKey: "profiles.max", IntValue: i64(1)},
		{FeatureKey: "profiles.max", IntValue: i64(2)},
		{FeatureKey: "test.flag", IntValue: i64(1)},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 errors, got %+v", got)
	}
	if got[0].Code != LimitCodeDuplicate || got[1].Field != "limits.test.flag" {
		t.Fatalf("unexpected errors %+v", got)
	}
}

// TEC-942: a plan must define every feature; each missing one is a required error on its field.
func TestValidateLimits_EveryFeatureRequired(t *testing.T) {
	for _, limits := range [][]PlanLimitInput{nil, {}} {
		got := ValidateLimits(testValueTypes, limits)
		if len(got) != 2 || got[0].Field != "limits.profiles.max" || got[1].Field != "limits.test.flag" {
			t.Fatalf("want required errors for both features, got %+v", got)
		}
		for _, e := range got {
			if e.Code != LimitCodeRequired || e.Message == "" {
				t.Fatalf("want code %q with a message, got %+v", LimitCodeRequired, e)
			}
		}
	}
	got := ValidateLimits(testValueTypes, []PlanLimitInput{{FeatureKey: "profiles.max", IntValue: i64(1)}})
	if len(got) != 1 || got[0].Field != "limits.test.flag" || got[0].Code != LimitCodeRequired {
		t.Fatalf("want one required error on limits.test.flag, got %+v", got)
	}
}

func TestValidationErrorIsInvalidRequest(t *testing.T) {
	err := error(&ValidationError{Fields: []FieldError{{Field: "limits.x", Message: "m"}}})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("ValidationError must match ErrInvalidRequest")
	}
}

// The admin UI and API clients send snake_case keys; the input must decode them.
func TestPlanLimitInputDecodesSnakeCase(t *testing.T) {
	var got []PlanLimitInput
	body := `[{"feature_key":"profiles.max","int_value":5},{"feature_key":"test.flag","bool_value":true}]`
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got[0].FeatureKey != "profiles.max" || got[0].IntValue == nil || *got[0].IntValue != 5 {
		t.Fatalf("int limit decoded wrong: %+v", got[0])
	}
	if got[1].FeatureKey != "test.flag" || got[1].BoolValue == nil || !*got[1].BoolValue {
		t.Fatalf("bool limit decoded wrong: %+v", got[1])
	}
}
