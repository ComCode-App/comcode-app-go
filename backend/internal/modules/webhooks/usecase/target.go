package usecase

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Endpoint URLs must be public HTTPS addresses. Private, loopback, link-local and metadata
// ranges are refused both when the URL is saved (literal hosts) and when it is dialled (after
// DNS), so a webhook cannot be pointed at the API's own network (SSRF).
var ErrInvalidURL = errors.New("webhook url must be a public https address")

// ValidateURL checks the saved form of an endpoint URL.
func ValidateURL(raw string, allowInsecure bool) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return "", ErrInvalidURL
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !allowInsecure {
			return "", ErrInvalidURL
		}
	default:
		return "", ErrInvalidURL
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		if !allowInsecure {
			return "", ErrInvalidURL
		}
	}
	if ip := net.ParseIP(host); ip != nil && !allowInsecure && !isPublicIP(ip) {
		return "", ErrInvalidURL
	}
	if len(raw) > 2048 {
		return "", ErrInvalidURL
	}
	return u.String(), nil
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	// IPv6 forms that embed an IPv4 address are judged by that address.
	if ip.To4() == nil && len(ip) == net.IPv6len {
		switch {
		case ip[0] == 0xfc || ip[0] == 0xfd: // fc00::/7 unique local
			return false
		case ip[0] == 0x20 && ip[1] == 0x02: // 2002::/16 6to4
			return isPublicIP(net.IPv4(ip[2], ip[3], ip[4], ip[5]))
		case ip[0] == 0 && ip[1] == 0x64 && ip[2] == 0xff && ip[3] == 0x9b: // 64:ff9b::/96 NAT64
			return isPublicIP(net.IPv4(ip[12], ip[13], ip[14], ip[15]))
		}
		return true
	}
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	switch {
	case v4[0] == 0: // 0.0.0.0/8 "this network"
		return false
	case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127: // carrier-grade NAT
		return false
	case v4[0] == 192 && v4[1] == 0 && v4[2] == 0: // protocol assignments
		return false
	case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19): // benchmarking
		return false
	case v4[0] >= 240: // reserved and broadcast
		return false
	}
	return true
}

// NewHTTPClient returns the delivery client: short timeouts, no redirects (a 3xx counts as a
// failure so a redirect cannot bounce the request to an internal host) and a dial-time check of
// the resolved IP.
func NewHTTPClient(allowInsecure bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !allowInsecure {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("webhook: refused dial to %s", host)
			}
			return nil
		}
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          20,
			IdleConnTimeout:       30 * time.Second,
			Proxy:                 nil,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
