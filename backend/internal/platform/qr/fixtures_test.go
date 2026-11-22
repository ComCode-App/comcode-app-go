package qr

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Golden fixtures shared with the TypeScript engine
// (frontend/src/features/route-qr/lib/qr-engine.test.mjs). Regenerate after an intentional
// geometry change with `make qr-fixtures` (go test ./internal/platform/qr -run TestGoldenFixtures -update).
var update = flag.Bool("update", false, "rewrite testdata golden fixtures")

type fixture struct {
	Name     string          `json:"name"`
	Matrix   []string        `json:"matrix"`
	Size     int             `json:"size"`
	LogoHref string          `json:"logo_href"`
	Style    json.RawMessage `json:"style"`
	SVG      string          `json:"svg"`
}

// Alphanumeric mode keeps the fixture symbol at version 1 (21x21) and the files small.
const fixturePayload = "HTTPS://CC.TEST/F"

// A 1x1 transparent PNG: fixtures pin the geometry, not logo pixels.
const fixtureLogo = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

func fixtureCases() map[string]map[string]any {
	cases := map[string]map[string]any{}
	base := func() map[string]any {
		return map[string]any{"foreground": "#000000", "background": "#ffffff", "quiet_zone": 4}
	}
	mirror := []map[string]any{{}, {"flip_h": true}, {"flip_v": true}}
	for i, body := range BodyShapes {
		s := base()
		s["body_shape"] = body
		s["eye_frame_shape"] = EyeFrameShapes[i%len(EyeFrameShapes)]
		s["eye_ball_shape"] = EyeBallShapes[i%len(EyeBallShapes)]
		switch i % 6 {
		case 1:
			s["foreground"] = "#1F2937"
			s["eyes"] = []map[string]any{
				{"frame_color": "#7c2d12", "ball_color": "#1e3a8a"},
				{"frame_color": "#14532d", "ball_color": "#000000", "flip_h": true},
				{"frame_color": "#4c1d95", "ball_color": "#7c2d12", "flip_h": true, "flip_v": true},
			}
		case 2:
			s["fill_mode"] = FillLinear
			s["gradient_start"], s["gradient_end"], s["gradient_angle"] = "#1e3a8a", "#7c2d12", 45
			s["gradient_on_eyes"] = true
			s["eyes"] = mirror
		case 3:
			s["fill_mode"] = FillRadial
			s["gradient_start"], s["gradient_end"] = "#000000", "#14532d"
			s["eyes"] = []map[string]any{
				{"frame_color": "#4c1d95", "ball_color": "#000000"},
				{"frame_color": "#4c1d95", "ball_color": "#000000", "flip_v": true},
				{"frame_color": "#4c1d95", "ball_color": "#000000"},
			}
		case 4:
			s["foreground"] = "#0b3d91"
			s["transparent_background"] = true
			s["quiet_zone"] = 6
		case 5:
			s["fill_mode"] = FillLinear
			s["gradient_start"], s["gradient_end"], s["gradient_angle"] = "#000000", "#1e3a8a", 270
			s["background"] = "#fff7ed"
			s["eyes"] = mirror
		}
		cases[fmt.Sprintf("body-%02d-%s", i, body)] = s
	}
	// Frames and balls beyond the body count, flipped both ways.
	for i := len(BodyShapes); i < len(EyeBallShapes) || i < len(EyeFrameShapes); i++ {
		s := base()
		s["eye_frame_shape"] = EyeFrameShapes[i%len(EyeFrameShapes)]
		s["eye_ball_shape"] = EyeBallShapes[i%len(EyeBallShapes)]
		s["eyes"] = []map[string]any{{"flip_h": true, "flip_v": true}, {"flip_v": true}, {"flip_h": true}}
		cases[fmt.Sprintf("eyes-%02d", i)] = s
	}
	// v1 documents: only module/eye; must keep their v1 look.
	for _, legacy := range [][2]string{{"square", "square"}, {"rounded", "circle"}, {"dots", "rounded"}} {
		s := base()
		s["module"], s["eye"] = legacy[0], legacy[1]
		cases["legacy-"+legacy[0]+"-"+legacy[1]] = s
	}
	logo := func(mode string, transparent bool, size int) map[string]any {
		s := base()
		s["body_shape"] = "fluid"
		s["logo_profile_uuid"] = "00000000-0000-4000-8000-000000000001"
		s["logo_field"] = "avatar"
		s["logo_mode"] = mode
		s["logo_size"] = size
		s["transparent_background"] = transparent
		return s
	}
	cases["logo-clean"] = logo(LogoClean, false, 22)
	cases["logo-clean-transparent"] = logo(LogoClean, true, 30)
	cases["logo-overlay"] = logo(LogoOverlay, false, 12)
	legacyLogo := base()
	legacyLogo["module"], legacyLogo["eye"] = "rounded", "rounded"
	legacyLogo["logo_profile_uuid"] = "00000000-0000-4000-8000-000000000002"
	legacyLogo["logo_field"] = "cover"
	cases["legacy-logo"] = legacyLogo
	return cases
}

func TestGoldenFixtures(t *testing.T) {
	dir := filepath.Join("testdata")
	if *update {
		rows, err := Matrix(fixturePayload, false)
		if err != nil {
			t.Fatal(err)
		}
		old, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		for _, f := range old {
			_ = os.Remove(f)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, raw := range fixtureCases() {
			styleJSON, _ := json.Marshal(raw)
			f := fixture{Name: name, Matrix: rows, Size: 512, Style: styleJSON}
			if _, ok := raw["logo_profile_uuid"]; ok {
				f.LogoHref = fixtureLogo
			}
			var s Style
			if err := json.Unmarshal(styleJSON, &s); err != nil {
				t.Fatal(err)
			}
			if err := s.Validate(); err != nil {
				t.Fatalf("%s: fixture style is invalid", name)
			}
			m, _ := ParseMatrix(rows)
			f.SVG = RenderSVG(m, s, f.Size, f.LogoHref)
			out, _ := json.MarshalIndent(f, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, name+".json"), append(out, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) == 0 {
		t.Fatal("no fixtures; run make qr-fixtures")
	}
	seen := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var f fixture
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		var s Style
		if err := json.Unmarshal(f.Style, &s); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		m, err := ParseMatrix(f.Matrix)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if got := RenderSVG(m, s, f.Size, f.LogoHref); got != f.SVG {
			t.Errorf("%s: SVG differs from the golden fixture (run make qr-fixtures after an intentional change)", f.Name)
		}
		n := s.Normalize()
		seen["body:"+n.BodyShape] = true
		seen["frame:"+n.EyeFrameShape] = true
		seen["ball:"+n.EyeBallShape] = true
		seen["fill:"+n.FillMode] = true
		if n.TransparentBackground {
			seen["transparent"] = true
		}
		if n.GradientOnEyes {
			seen["gradient-on-eyes"] = true
		}
		if n.LogoProfileUUID != nil {
			seen["logo:"+n.LogoMode] = true
		}
		for _, e := range n.Eyes {
			if e.FlipH {
				seen["flip-h"] = true
			}
			if e.FlipV {
				seen["flip-v"] = true
			}
			if e.FrameColor != "" || e.BallColor != "" {
				seen["eye-color"] = true
			}
		}
	}
	var missing []string
	need := []string{"fill:solid", "fill:linear", "fill:radial", "transparent", "gradient-on-eyes", "logo:clean", "logo:overlay", "flip-h", "flip-v", "eye-color"}
	for _, v := range BodyShapes {
		need = append(need, "body:"+v)
	}
	for _, v := range EyeFrameShapes {
		need = append(need, "frame:"+v)
	}
	for _, v := range EyeBallShapes {
		need = append(need, "ball:"+v)
	}
	for _, k := range need {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("fixtures do not cover: %s", strings.Join(missing, ", "))
	}
}
