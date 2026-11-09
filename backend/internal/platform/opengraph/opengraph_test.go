package opengraph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Fetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
	<title>Test Page Title</title>
	<meta property="og:title" content="OpenGraph Title" />
	<meta property="og:description" content="This is an awesome description." />
	<meta property="og:site_name" content="AwesomeSite" />
	<meta property="og:image" content="/images/og.png" />
	<link rel="icon" href="/favicon.ico" />
</head>
<body><h1>Hello World</h1></body>
</html>`))
	}))
	defer ts.Close()

	client := NewClient()
	meta, err := client.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Title != "OpenGraph Title" {
		t.Errorf("expected title 'OpenGraph Title', got %q", meta.Title)
	}
	if meta.Description != "This is an awesome description." {
		t.Errorf("expected description 'This is an awesome description.', got %q", meta.Description)
	}
	if meta.SiteName != "AwesomeSite" {
		t.Errorf("expected site_name 'AwesomeSite', got %q", meta.SiteName)
	}
}
