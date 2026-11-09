package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// MaxDocumentBytes caps a stored background document.
const MaxDocumentBytes = 32 << 10

// Kinds are the background kinds the studio produces.
var Kinds = map[string]struct{}{"solid": {}, "gradient": {}, "video": {}, "shader": {}}

// ShaderTypes mirrors frontend/src/features/background-studio/data/shader-types.ts.
var ShaderTypes = map[string]struct{}{
	"mesh-gradient": {}, "static-mesh-gradient": {}, "static-radial-gradient": {}, "grain-gradient": {},
	"warp": {}, "swirl": {}, "spiral": {}, "god-rays": {}, "pulsing-border": {}, "color-panels": {},
	"smoke-ring": {}, "dithering": {}, "dot-grid": {}, "dot-orbit": {}, "metaballs": {},
	"neuro-noise": {}, "simplex-noise": {}, "perlin-noise": {}, "voronoi": {}, "waves": {},
	"paper-texture": {}, "fluted-glass": {}, "water": {}, "image-dithering": {}, "halftone-dots": {},
	"halftone-cmyk": {}, "lens-distortion": {}, "liquid-metal": {}, "heatmap": {}, "gem-smoke": {},
}

// StaticShaderTypes are the shaders without motion (no speed/frame params in the studio
// registry). They count as basic backgrounds; every other shader is animated and premium
// (TEC-947). Mirrors frontend/src/features/background-studio/data/shader-tiers.json.
var StaticShaderTypes = []string{"static-mesh-gradient", "static-radial-gradient", "paper-texture", "image-dithering"}

// IsPremium reports whether a (validated) document needs the design.premium entitlement:
// animated shaders and videos. Solid colors, gradients and static shaders are free.
func IsPremium(raw json.RawMessage) bool {
	var doc struct {
		Kind       string `json:"kind"`
		ShaderType string `json:"shaderType"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	switch doc.Kind {
	case "video":
		return true
	case "shader":
		return !slices.Contains(StaticShaderTypes, doc.ShaderType)
	default:
		return false
	}
}

var ErrInvalidDocument = errors.New("invalid background document")

// safeToken covers colors (#hex, rgb()/hsl(), names), enum values and short labels. It cannot
// carry CSS breakouts (; { } : / \ quotes) or markup.
var safeToken = regexp.MustCompile(`^[a-zA-Z0-9 #%().,_+-]{0,64}$`)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDocument, fmt.Sprintf(format, args...))
}

// ValidateDocument checks a studio BackgroundDocument and returns its kind. Values end up in CSS
// (solid/gradient), video/img sources and shader uniforms on public pages, so strings must be
// safe tokens or https URLs, numbers must be finite, and nesting is shallow.
func ValidateDocument(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || len(raw) > MaxDocumentBytes {
		return "", invalid("document must be 1..%d bytes", MaxDocumentBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return "", invalid("document must be a JSON object")
	}
	kind, _ := doc["kind"].(string)
	if _, ok := Kinds[kind]; !ok {
		return "", invalid("unknown kind %q", kind)
	}
	if kind == "shader" {
		st, _ := doc["shaderType"].(string)
		if _, ok := ShaderTypes[st]; !ok {
			return "", invalid("unknown shaderType %q", st)
		}
	}
	params, _ := doc["params"].(map[string]any)
	if params == nil {
		return "", invalid("params must be an object")
	}
	if kind == "video" {
		src, _ := params["src"].(string)
		if !isHTTPSURL(src) {
			return "", invalid("video src must be an https URL")
		}
	}
	// Outside params, values are labels (name, id, createdAt, metadata): rendered as text, never
	// as CSS or URLs, so any printable text without markup is fine.
	for key, v := range doc {
		if key == "params" {
			continue
		}
		if err := checkText(key, v, 1); err != nil {
			return "", err
		}
	}
	for key, v := range params {
		if err := checkValue("params."+key, v, 1); err != nil {
			return "", err
		}
	}
	return kind, nil
}

func checkValue(path string, v any, depth int) error {
	if depth > 3 {
		return invalid("%s is nested too deeply", path)
	}
	switch val := v.(type) {
	case nil, bool:
		return nil
	case json.Number:
		if _, err := val.Float64(); err != nil {
			return invalid("%s is not a finite number", path)
		}
		return nil
	case string:
		if safeToken.MatchString(val) || isHTTPSURL(val) {
			return nil
		}
		return invalid("%s has an unsupported value", path)
	case []any:
		if len(val) > 32 {
			return invalid("%s has too many items", path)
		}
		for i, item := range val {
			if err := checkValue(fmt.Sprintf("%s[%d]", path, i), item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(val) > 64 {
			return invalid("%s has too many keys", path)
		}
		for k, item := range val {
			if err := checkValue(path+"."+k, item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return invalid("%s has an unsupported type", path)
	}
}

func checkText(path string, v any, depth int) error {
	if depth > 3 {
		return invalid("%s is nested too deeply", path)
	}
	switch val := v.(type) {
	case nil, bool, json.Number:
		return nil
	case string:
		if len(val) > 500 || strings.ContainsAny(val, "<>") || strings.ContainsFunc(val, unicode.IsControl) {
			return invalid("%s has an unsupported value", path)
		}
		return nil
	case []any:
		if len(val) > 32 {
			return invalid("%s has too many items", path)
		}
		for i, item := range val {
			if err := checkText(fmt.Sprintf("%s[%d]", path, i), item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(val) > 32 {
			return invalid("%s has too many keys", path)
		}
		for k, item := range val {
			if err := checkText(path+"."+k, item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return invalid("%s has an unsupported type", path)
	}
}

func isHTTPSURL(s string) bool {
	if len(s) > 2048 || strings.ContainsAny(s, "\"'<>\\ ") {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
