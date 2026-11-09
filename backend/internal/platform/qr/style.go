package qr

import (
	"errors"
	"image/color"
	"math"
	"regexp"
	"strconv"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid QR style or render options")
var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Style selects appearance; logos reference existing account profile media, never URLs or keys.
type Style struct {
	Foreground      string     `json:"foreground"`
	Background      string     `json:"background"`
	Module          string     `json:"module"`
	Eye             string     `json:"eye"`
	QuietZone       int        `json:"quiet_zone"`
	LogoProfileUUID *uuid.UUID `json:"logo_profile_uuid"`
	LogoField       string     `json:"logo_field"`
}

func DefaultStyle() Style {
	return Style{Foreground: "#000000", Background: "#ffffff", Module: "square", Eye: "square", QuietZone: 4}
}

// Validate requires dark modules on a light background and a conservative 4.5:1 contrast.
func (s Style) Validate() error {
	if !hexColor.MatchString(s.Foreground) || !hexColor.MatchString(s.Background) {
		return ErrInvalid
	}
	fg, bg := luminance(parseColor(s.Foreground)), luminance(parseColor(s.Background))
	if (bg+0.05)/(fg+0.05) < 4.5 {
		return ErrInvalid
	}
	switch s.Module {
	case "square", "rounded", "dots":
	default:
		return ErrInvalid
	}
	switch s.Eye {
	case "square", "rounded", "circle":
	default:
		return ErrInvalid
	}
	if s.QuietZone < 4 || s.QuietZone > 16 {
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
