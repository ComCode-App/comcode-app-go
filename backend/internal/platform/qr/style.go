package qr

import (
	"encoding/json"
	"errors"
	"image/color"
	"math"
	"regexp"
	"strconv"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid QR style or render options")
var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// MinContrast is the directional (background over dark color) WCAG ratio every dark color needs.
const MinContrast = 4.5

// Logo size bounds, as a percentage of the symbol width (without the quiet zone).
const (
	LogoSizeMin     = 10
	LogoSizeMax     = 30
	LogoSizeDefault = 15
)

// Fill modes of the body (and, with gradient_on_eyes, of eyes without their own color).
const (
	FillSolid  = "solid"
	FillLinear = "linear"
	FillRadial = "radial"
)

// Logo modes: clean removes the modules under the logo (plus a one-module margin); overlay
// keeps every module and draws the logo on top.
const (
	LogoClean   = "clean"
	LogoOverlay = "overlay"
)

// EyeStyle is one finder pattern ("eye"): top-left, top-right, bottom-left in that order.
// Empty colors inherit the body (foreground, or the gradient with gradient_on_eyes).
type EyeStyle struct {
	FlipH      bool   `json:"flip_h"`
	FlipV      bool   `json:"flip_v"`
	FrameColor string `json:"frame_color"`
	BallColor  string `json:"ball_color"`
}

// Style selects appearance; logos reference existing account profile media, never URLs or keys.
//
// Version 1 stored only foreground/background/module/eye/quiet_zone and the logo reference.
// Those fields stay; Normalize maps them onto the v2 shape fields when the v2 fields are empty,
// so a stored v1 style renders exactly as before.
type Style struct {
	Foreground            string      `json:"foreground"`
	Background            string      `json:"background"`
	TransparentBackground bool        `json:"transparent_background"`
	Module                string      `json:"module"`
	Eye                   string      `json:"eye"`
	BodyShape             string      `json:"body_shape"`
	EyeFrameShape         string      `json:"eye_frame_shape"`
	EyeBallShape          string      `json:"eye_ball_shape"`
	Eyes                  [3]EyeStyle `json:"eyes"`
	FillMode              string      `json:"fill_mode"`
	GradientStart         string      `json:"gradient_start"`
	GradientEnd           string      `json:"gradient_end"`
	GradientAngle         int         `json:"gradient_angle"`
	GradientOnEyes        bool        `json:"gradient_on_eyes"`
	QuietZone             int         `json:"quiet_zone"`
	LogoProfileUUID       *uuid.UUID  `json:"logo_profile_uuid"`
	LogoField             string      `json:"logo_field"`
	LogoSize              int         `json:"logo_size"`
	LogoMode              string      `json:"logo_mode"`
}

// legacyStyle holds the v1 defaults a stored style is decoded over ('{}' is the column default).
func legacyStyle() Style {
	return Style{Foreground: "#000000", Background: "#ffffff", Module: "square", Eye: "square", QuietZone: 4}
}

// DefaultStyle is black on white, square shapes, a four-module quiet zone and no logo.
func DefaultStyle() Style {
	return legacyStyle().Normalize()
}

// ParseStored decodes a stored routes.qr_style document (v1 or v2) into a normalized style.
func ParseStored(raw []byte) (Style, error) {
	style := legacyStyle()
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &style); err != nil {
			return Style{}, err
		}
	}
	return style.Normalize(), nil
}

// Legacy v1 values and their v2 equivalents. These pairs render identically to v1.
var (
	legacyModuleBody = map[string]string{"square": "square", "rounded": "rounded-square", "dots": "circle"}
	legacyEyeFrame   = map[string]string{"square": "square", "rounded": "rounded", "circle": "circle"}
	legacyEyeBall    = map[string]string{"square": "square", "rounded": "rounded", "circle": "circle"}
)

func legacyModuleFor(body string) string {
	for k, v := range legacyModuleBody {
		if v == body {
			return k
		}
	}
	return "square"
}

func legacyEyeFor(frame, ball string) string {
	for k, v := range legacyEyeFrame {
		if v == frame && legacyEyeBall[k] == ball {
			return k
		}
	}
	return "square"
}

// Normalize fills v2 fields from v1 fields (and v1 from v2) plus defaults. It never rejects;
// Validate does. Colors are lower-cased so equal styles compare equal.
func (s Style) Normalize() Style {
	// An omitted v1 field means its default (square), as for a v2-only client.
	module, eye := s.Module, s.Eye
	if module == "" {
		module = "square"
	}
	if eye == "" {
		eye = "square"
	}
	if s.BodyShape == "" {
		s.BodyShape = legacyModuleBody[module]
	}
	if s.Module == "" {
		s.Module = legacyModuleFor(s.BodyShape)
	}
	if s.EyeFrameShape == "" {
		s.EyeFrameShape = legacyEyeFrame[eye]
	}
	if s.EyeBallShape == "" {
		s.EyeBallShape = legacyEyeBall[eye]
	}
	if s.Eye == "" {
		s.Eye = legacyEyeFor(s.EyeFrameShape, s.EyeBallShape)
	}
	if s.FillMode == "" {
		s.FillMode = FillSolid
	}
	if s.GradientStart == "" {
		s.GradientStart = s.Foreground
	}
	if s.GradientEnd == "" {
		s.GradientEnd = s.Foreground
	}
	if s.LogoSize == 0 {
		s.LogoSize = LogoSizeDefault
	}
	if s.LogoMode == "" {
		s.LogoMode = LogoClean
	}
	lower := func(v string) string {
		if hexColor.MatchString(v) {
			return toLowerHex(v)
		}
		return v
	}
	s.Foreground, s.Background = lower(s.Foreground), lower(s.Background)
	s.GradientStart, s.GradientEnd = lower(s.GradientStart), lower(s.GradientEnd)
	for i := range s.Eyes {
		s.Eyes[i].FrameColor, s.Eyes[i].BallColor = lower(s.Eyes[i].FrameColor), lower(s.Eyes[i].BallColor)
	}
	return s
}

func toLowerHex(v string) string {
	b := []byte(v)
	for i, c := range b {
		if c >= 'A' && c <= 'F' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// Validate normalizes, then requires known shapes and dark-on-light colors: every dark color
// (foreground, both gradient colors when a gradient is on, every explicit eye color) needs a
// 4.5:1 directional contrast against the background. A transparent background is judged as
// white, the light surface it is meant to be printed or shown on.
func (s Style) Validate() error {
	s = s.Normalize()
	if !hexColor.MatchString(s.Foreground) || !hexColor.MatchString(s.Background) {
		return ErrInvalid
	}
	bg := s.Background
	if s.TransparentBackground {
		bg = "#ffffff"
	}
	dark := []string{s.Foreground}
	switch s.FillMode {
	case FillSolid:
	case FillLinear, FillRadial:
		dark = append(dark, s.GradientStart, s.GradientEnd)
	default:
		return ErrInvalid
	}
	if !hexColor.MatchString(s.GradientStart) || !hexColor.MatchString(s.GradientEnd) {
		return ErrInvalid
	}
	for _, e := range s.Eyes {
		for _, c := range []string{e.FrameColor, e.BallColor} {
			if c != "" {
				dark = append(dark, c)
			}
		}
	}
	for _, c := range dark {
		if !hexColor.MatchString(c) || Contrast(c, bg) < MinContrast {
			return ErrInvalid
		}
	}
	if _, ok := legacyModuleBody[s.Module]; !ok {
		return ErrInvalid
	}
	if _, ok := legacyEyeFrame[s.Eye]; !ok {
		return ErrInvalid
	}
	if !IsBodyShape(s.BodyShape) || !IsEyeFrameShape(s.EyeFrameShape) || !IsEyeBallShape(s.EyeBallShape) {
		return ErrInvalid
	}
	if s.GradientAngle < 0 || s.GradientAngle > 359 {
		return ErrInvalid
	}
	if s.QuietZone < 4 || s.QuietZone > 16 {
		return ErrInvalid
	}
	if s.LogoSize < LogoSizeMin || s.LogoSize > LogoSizeMax {
		return ErrInvalid
	}
	if s.LogoMode != LogoClean && s.LogoMode != LogoOverlay {
		return ErrInvalid
	}
	if s.LogoProfileUUID == nil {
		if s.LogoField != "" {
			return ErrInvalid
		}
	} else if *s.LogoProfileUUID == uuid.Nil || (s.LogoField != "avatar" && s.LogoField != "cover") {
		return ErrInvalid
	}
	return nil
}

// Contrast is the directional ratio (background + 0.05) / (dark + 0.05). Light-on-dark
// ("inverted") pairs score below 1 and are rejected because many scanners fail on them.
func Contrast(dark, background string) float64 {
	fg, bg := luminance(parseColor(dark)), luminance(parseColor(background))
	return (bg + 0.05) / (fg + 0.05)
}

func parseColor(s string) color.RGBA {
	v, _ := strconv.ParseUint(s[1:], 16, 32)
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func luminance(c color.RGBA) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}
