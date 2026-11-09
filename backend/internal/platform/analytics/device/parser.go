package device

import (
	"regexp"
	"strings"
)

// Info holds parsed client device, browser, and OS metadata.
type Info struct {
	DeviceType     string `json:"device_type"`
	DeviceBrand    string `json:"device_brand"`
	DeviceModel    string `json:"device_model"`
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
}

// UserAgentParser extracts client device and browser parameters from User-Agent strings.
type UserAgentParser interface {
	Parse(userAgent string) Info
}

// RegexParser is a lightweight, zero-dependency User-Agent parser.
type RegexParser struct{}

func NewParser() *RegexParser {
	return &RegexParser{}
}

var (
	botRegex    = regexp.MustCompile(`(?i)(bot|crawler|spider|googlebot|bingbot|slurp|duckduckbot|baiduspider|yandexbot|facebookexternalhit|whatsapp|telegrambot|twitterbot)`)
	tabletRegex = regexp.MustCompile(`(?i)(ipad|tablet|playbook|silk)`)
	mobileRegex = regexp.MustCompile(`(?i)(mobi|iphone|ipod|android|blackberry|windows phone|opera mini|iemobile)`)

	chromeRegex  = regexp.MustCompile(`(?i)Chrome/([0-9.]+)`)
	safariRegex  = regexp.MustCompile(`(?i)Version/([0-9.]+).*Safari`)
	firefoxRegex = regexp.MustCompile(`(?i)Firefox/([0-9.]+)`)
	edgeRegex    = regexp.MustCompile(`(?i)Edg[eA]?/([0-9.]+)`)
	operaRegex   = regexp.MustCompile(`(?i)(OPR|Opera)/([0-9.]+)`)

	iosRegex     = regexp.MustCompile(`(?i)OS ([0-9_]+) like Mac OS X`)
	androidRegex = regexp.MustCompile(`(?i)Android ([0-9.]+)`)
	windowsRegex = regexp.MustCompile(`(?i)Windows NT ([0-9.]+)`)
	macRegex     = regexp.MustCompile(`(?i)Mac OS X ([0-9_.]+)`)
)

// Parse extracts device, browser, and operating system attributes from a User-Agent string.
func (p *RegexParser) Parse(ua string) Info {
	if ua == "" {
		return Info{DeviceType: "unknown"}
	}

	info := Info{
		DeviceType:  "desktop",
		DeviceBrand: "Generic",
	}

	uaLower := strings.ToLower(ua)

	// 1. Device Type & Brand Detection
	if botRegex.MatchString(ua) {
		info.DeviceType = "bot"
		info.DeviceBrand = "Bot"
	} else if tabletRegex.MatchString(ua) || (strings.Contains(uaLower, "android") && !strings.Contains(uaLower, "mobi")) {
		info.DeviceType = "tablet"
		if strings.Contains(uaLower, "ipad") {
			info.DeviceBrand = "Apple"
			info.DeviceModel = "iPad"
		}
	} else if mobileRegex.MatchString(ua) {
		info.DeviceType = "mobile"
		if strings.Contains(uaLower, "iphone") {
			info.DeviceBrand = "Apple"
			info.DeviceModel = "iPhone"
		} else if strings.Contains(uaLower, "samsung") {
			info.DeviceBrand = "Samsung"
		}
	}

	// 2. OS & OS Version Detection
	if match := iosRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.OS = "iOS"
		info.OSVersion = strings.ReplaceAll(match[1], "_", ".")
		if info.DeviceBrand == "Generic" {
			info.DeviceBrand = "Apple"
		}
	} else if match := androidRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.OS = "Android"
		info.OSVersion = match[1]
	} else if match := macRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.OS = "macOS"
		info.OSVersion = strings.ReplaceAll(match[1], "_", ".")
		info.DeviceBrand = "Apple"
	} else if match := windowsRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.OS = "Windows"
		info.OSVersion = match[1]
	} else if strings.Contains(strings.ToLower(ua), "linux") {
		info.OS = "Linux"
	}

	// 3. Browser & Browser Version Detection
	if match := edgeRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.Browser = "Edge"
		info.BrowserVersion = match[1]
	} else if match := operaRegex.FindStringSubmatch(ua); len(match) > 2 {
		info.Browser = "Opera"
		info.BrowserVersion = match[2]
	} else if match := chromeRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.Browser = "Chrome"
		info.BrowserVersion = match[1]
	} else if match := safariRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.Browser = "Safari"
		info.BrowserVersion = match[1]
	} else if match := firefoxRegex.FindStringSubmatch(ua); len(match) > 1 {
		info.Browser = "Firefox"
		info.BrowserVersion = match[1]
	} else {
		info.Browser = "Other"
	}

	return info
}
