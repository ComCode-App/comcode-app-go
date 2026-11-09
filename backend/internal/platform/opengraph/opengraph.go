package opengraph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/otiai10/opengraph/v2"
)

var (
	ErrInvalidURL = errors.New("invalid or unparseable URL")
	ErrFetchFail  = errors.New("failed to fetch link metadata")
)

type LinkMetadata struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SiteName    string `json:"site_name"`
	Favicon     string `json:"favicon"`
	ImageURL    string `json:"image_url"`
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
	}
}

func (c *Client) Fetch(ctx context.Context, targetURL string) (*LinkMetadata, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, ErrInvalidURL
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" {
		return nil, ErrInvalidURL
	}

	intent := opengraph.Intent{
		Context:    ctx,
		HTTPClient: c.httpClient,
		Strict:     false,
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (compatible; ComCode-Bot/1.0; +https://comcode.app)",
			"Accept-Language": "en-US,en;q=0.9,tr;q=0.8",
		},
	}

	og, err := opengraph.Fetch(targetURL, intent)
	if err != nil {
		// Fallback to basic domain metadata if full OG scrape fails
		return &LinkMetadata{
			URL:      targetURL,
			Title:    parsed.Host,
			SiteName: parsed.Host,
			Favicon:  fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=128", parsed.Host),
		}, nil
	}

	meta := &LinkMetadata{
		URL:         targetURL,
		Title:       og.Title,
		Description: og.Description,
		SiteName:    og.SiteName,
	}

	if meta.Title == "" {
		meta.Title = parsed.Host
	}
	if meta.SiteName == "" {
		meta.SiteName = parsed.Host
	}

	if len(og.Image) > 0 && og.Image[0].URL != "" {
		imgURL := og.Image[0].URL
		if strings.HasPrefix(imgURL, "//") {
			imgURL = parsed.Scheme + ":" + imgURL
		} else if strings.HasPrefix(imgURL, "/") {
			imgURL = fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, imgURL)
		}
		meta.ImageURL = imgURL
	}

	if og.Favicon.URL != "" {
		fav := og.Favicon.URL
		if strings.HasPrefix(fav, "//") {
			fav = parsed.Scheme + ":" + fav
		} else if strings.HasPrefix(fav, "/") {
			fav = fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, fav)
		}
		meta.Favicon = fav
	} else {
		meta.Favicon = fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=128", parsed.Host)
	}

	return meta, nil
}
