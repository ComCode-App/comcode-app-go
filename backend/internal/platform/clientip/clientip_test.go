package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolve(t *testing.T) {
	trusted, err := ParseTrusted(DefaultTrustedProxies)
	if err != nil {
		t.Fatal(err)
	}
	rs := New(trusted)

	cases := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{"untrusted peer ignores spoofed header", "203.0.113.9:4000", []string{"1.1.1.1"}, "203.0.113.9"},
		{"trusted peer without header", "10.0.0.5:4000", nil, "10.0.0.5"},
		{"trusted peer uses forwarded client", "10.0.0.5:4000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"client-supplied prefix is ignored", "10.0.0.5:4000", []string{"1.1.1.1, 198.51.100.7"}, "198.51.100.7"},
		{"trusted hops are skipped", "127.0.0.1:4000", []string{"198.51.100.7, 172.18.0.3"}, "198.51.100.7"},
		{"multiple header lines", "10.0.0.5:4000", []string{"1.1.1.1", "198.51.100.7"}, "198.51.100.7"},
		{"malformed hop stops the walk", "10.0.0.5:4000", []string{"198.51.100.7, garbage"}, "10.0.0.5"},
		{"ipv6 client", "[::1]:4000", []string{"2001:db8::1"}, "2001:db8::1"},
		{"ipv4-mapped peer", "[::ffff:10.0.0.5]:4000", []string{"198.51.100.7"}, "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := rs.Resolve(r).String(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNoTrustedProxies(t *testing.T) {
	rs := New(nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:4000"
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := rs.Resolve(r).String(); got != "127.0.0.1" {
		t.Fatalf("got %s", got)
	}
}

func TestMiddlewareRewritesRemoteAddr(t *testing.T) {
	trusted, _ := ParseTrusted("10.0.0.0/8")
	rs := New(trusted)
	var gotRemote, gotFrom string
	h := rs.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRemote = r.RemoteAddr
		gotFrom = String(r)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.1.2.3:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if gotFrom != "198.51.100.7" || gotRemote != "198.51.100.7:0" {
		t.Fatalf("from=%s remote=%s", gotFrom, gotRemote)
	}
}

func TestParseTrustedRejectsGarbage(t *testing.T) {
	if _, err := ParseTrusted("10.0.0.0/8, nope"); err == nil {
		t.Fatal("expected error")
	}
}
