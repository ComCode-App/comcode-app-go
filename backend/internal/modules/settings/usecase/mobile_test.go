package usecase

import "testing"

func sp(s string) *string { return &s }

func TestPatchInputValidateMobile(t *testing.T) {
	cases := []struct {
		name    string
		in      PatchInput
		current Settings
		fields  []string
	}{
		{"empty patch", PatchInput{}, Settings{}, nil},
		{"valid values", PatchInput{
			MobileMinVersion: sp("1.2"), MobileLatestVersion: sp("1.3.0"),
			MobileIOSStoreURL: sp("https://apps.apple.com/app/id1"), MobileAndroidStoreURL: sp(""),
		}, Settings{}, nil},
		{"clearing is allowed", PatchInput{MobileMinVersion: sp(""), MobileIOSStoreURL: sp(" ")}, Settings{MobileMinVersion: "1.0.0"}, nil},
		{"bad version", PatchInput{MobileMinVersion: sp("v1.2.3-beta")}, Settings{}, []string{"mobile_min_version"}},
		{"latest below min", PatchInput{MobileMinVersion: sp("2.0.0"), MobileLatestVersion: sp("1.9.9")}, Settings{}, []string{"mobile_latest_version"}},
		{"latest below stored min", PatchInput{MobileLatestVersion: sp("1.0.0")}, Settings{MobileMinVersion: "1.5.0"}, []string{"mobile_latest_version"}},
		{"http store url", PatchInput{MobileAndroidStoreURL: sp("http://play.google.com/x")}, Settings{}, []string{"mobile_android_store_url"}},
		{"javascript url", PatchInput{MobileIOSStoreURL: sp("javascript:alert(1)")}, Settings{}, []string{"mobile_ios_store_url"}},
		{"credentials in url", PatchInput{MobileIOSStoreURL: sp("https://u:p@apps.apple.com/x")}, Settings{}, []string{"mobile_ios_store_url"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.in.Validate(tc.current)
			if len(errs) != len(tc.fields) {
				t.Fatalf("errors = %+v, want fields %v", errs, tc.fields)
			}
			for i, f := range tc.fields {
				if errs[i].Field != f {
					t.Fatalf("errors = %+v, want fields %v", errs, tc.fields)
				}
			}
		})
	}
}
