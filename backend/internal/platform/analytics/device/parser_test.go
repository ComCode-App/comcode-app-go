package device_test

import (
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/device"
)

func TestRegexParser_Parse(t *testing.T) {
	parser := device.NewParser()

	tests := []struct {
		name        string
		ua          string
		wantType    string
		wantOS      string
		wantBrowser string
	}{
		{
			name:        "Mac Chrome Desktop",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
			wantType:    "desktop",
			wantOS:      "macOS",
			wantBrowser: "Chrome",
		},
		{
			name:        "iPhone Safari Mobile",
			ua:          "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
			wantType:    "mobile",
			wantOS:      "iOS",
			wantBrowser: "Safari",
		},
		{
			name:        "iPad Tablet",
			ua:          "Mozilla/5.0 (iPad; CPU OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
			wantType:    "tablet",
			wantOS:      "iOS",
			wantBrowser: "Safari",
		},
		{
			name:        "Android Mobile Chrome",
			ua:          "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.122 Mobile Safari/537.36",
			wantType:    "mobile",
			wantOS:      "Android",
			wantBrowser: "Chrome",
		},
		{
			name:        "Googlebot",
			ua:          "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantType:    "bot",
			wantOS:      "",
			wantBrowser: "Other",
		},
		{
			name:        "Windows 11 Edge",
			ua:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0",
			wantType:    "desktop",
			wantOS:      "Windows",
			wantBrowser: "Edge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.Parse(tt.ua)
			if got.DeviceType != tt.wantType {
				t.Errorf("DeviceType: got %q, want %q", got.DeviceType, tt.wantType)
			}
			if tt.wantOS != "" && got.OS != tt.wantOS {
				t.Errorf("OS: got %q, want %q", got.OS, tt.wantOS)
			}
			if tt.wantBrowser != "" && got.Browser != tt.wantBrowser {
				t.Errorf("Browser: got %q, want %q", got.Browser, tt.wantBrowser)
			}
		})
	}
}
