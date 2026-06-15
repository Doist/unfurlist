package unfurlist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestFetchOembedVideoExtractsStructuredEmbed(t *testing.T) {
	html := `<iframe width="200" height="113" src="https://www.youtube.com/embed/MESYWwg-98I?feature=oembed"></iframe>`
	body := mustOembedJSON(t, map[string]any{
		"type":             "video",
		"title":            "Hajmo Bosno Bosno",
		"provider_name":    "YouTube",
		"thumbnail_url":    "https://i.ytimg.com/vi/MESYWwg-98I/hqdefault.jpg",
		"thumbnail_width":  480,
		"thumbnail_height": 360,
		"width":            200,
		"height":           113,
		"html":             html,
	})
	res, err := fetchOembed(context.Background(), "https://www.youtube.com/oembed", staticOembedResponse(body), defaultEmbedHostAllowlist)
	if err != nil {
		t.Fatal(err)
	}
	if res.EmbedURL != "https://www.youtube.com/embed/MESYWwg-98I?feature=oembed" {
		t.Fatalf("unexpected EmbedURL: %q", res.EmbedURL)
	}
	if res.EmbedWidth != 200 || res.EmbedHeight != 113 {
		t.Fatalf("unexpected embed dimensions: %dx%d", res.EmbedWidth, res.EmbedHeight)
	}
	if res.ProviderName != "YouTube" || res.SiteName != "YouTube" {
		t.Fatalf("unexpected provider fields: provider=%q site=%q", res.ProviderName, res.SiteName)
	}
	if res.Image != "https://i.ytimg.com/vi/MESYWwg-98I/hqdefault.jpg" || res.ImageWidth != 480 || res.ImageHeight != 360 {
		t.Fatalf("unexpected thumbnail: %q %dx%d", res.Image, res.ImageWidth, res.ImageHeight)
	}
	if res.HTML != html {
		t.Fatalf("unexpected raw oEmbed HTML: %q", res.HTML)
	}
}

func TestFetchOembedVideoExtractsWrappedIframe(t *testing.T) {
	body := mustOembedJSON(t, map[string]any{
		"type":          "video",
		"title":         "Quoting Demo Review",
		"provider_name": "Supercut",
		"width":         3652,
		"height":        2132,
		"html": `<div style="position: relative; padding-bottom: 58.38%">
			<iframe src="https://supercut.ai/embed/doist/Q5JL6yXD5m7i5QjiOZ2RR9"></iframe>
		</div>`,
	})
	res, err := fetchOembed(context.Background(), "https://supercut.ai/oembed", staticOembedResponse(body), defaultEmbedHostAllowlist)
	if err != nil {
		t.Fatal(err)
	}
	if res.EmbedURL != "https://supercut.ai/embed/doist/Q5JL6yXD5m7i5QjiOZ2RR9" {
		t.Fatalf("unexpected EmbedURL: %q", res.EmbedURL)
	}
}

func TestFetchOembedResolvesRelativeIframeAgainstFinalResponseURL(t *testing.T) {
	body := mustOembedJSON(t, map[string]any{
		"type":          "video",
		"title":         "Redirected embed",
		"provider_name": "Example",
		"html":          `<iframe src="/embed/one"></iframe>`,
	})
	fn := func(_ context.Context, endpointURL string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, endpointURL, nil)
		if err != nil {
			return nil, err
		}
		resp := testHTTPResponse(req, http.StatusOK, "application/json", body)
		resp.Request.URL = mustParseURL(t, "https://player.example.com/oembed")
		return resp, nil
	}
	res, err := fetchOembed(context.Background(), "https://example.com/oembed", fn, []string{"player.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.EmbedURL != "https://player.example.com/embed/one" {
		t.Fatalf("unexpected EmbedURL: %q", res.EmbedURL)
	}
}

func TestFetchOembedRichDoesNotExtractIframe(t *testing.T) {
	html := `<iframe src="https://www.youtube.com/embed/MESYWwg-98I"></iframe>`
	body := mustOembedJSON(t, map[string]any{
		"type":          "rich",
		"title":         "Rich embed",
		"provider_name": "Example",
		"html":          html,
	})
	res, err := fetchOembed(context.Background(), "https://example.com/oembed", staticOembedResponse(body), defaultEmbedHostAllowlist)
	if err != nil {
		t.Fatal(err)
	}
	if res.EmbedURL != "" {
		t.Fatalf("rich oEmbed should not produce EmbedURL, got %q", res.EmbedURL)
	}
	if res.HTML != html {
		t.Fatalf("unexpected raw oEmbed HTML: %q", res.HTML)
	}
}

func TestFetchOembedVideoKeepsMetadataWhenIframeIsUnsafe(t *testing.T) {
	html := `<script src="https://example.com/embed.js"></script>`
	body := mustOembedJSON(t, map[string]any{
		"type":          "video",
		"title":         "Script-only video",
		"provider_name": "Example",
		"thumbnail_url": "https://example.com/thumb.jpg",
		"html":          html,
	})
	res, err := fetchOembed(context.Background(), "https://example.com/oembed", staticOembedResponse(body), defaultEmbedHostAllowlist)
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Script-only video" || res.Image != "https://example.com/thumb.jpg" || res.HTML != html {
		t.Fatalf("unexpected metadata: %#v", res)
	}
	if res.EmbedURL != "" {
		t.Fatalf("unsafe oEmbed HTML should not produce EmbedURL, got %q", res.EmbedURL)
	}
}

func TestExtractOembedIframeURLRejectsUnsafeShapes(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantErr error
	}{
		{
			name:    "script without iframe",
			html:    `<blockquote>hello</blockquote><script src="https://example.com/embed.js"></script>`,
			wantErr: errUnsafeEmbedHTML,
		},
		{
			name:    "no iframe",
			html:    `<div>hello</div>`,
			wantErr: errNoIframe,
		},
		{
			name:    "multiple iframes",
			html:    `<iframe src="https://www.youtube.com/embed/one"></iframe><iframe src="https://www.youtube.com/embed/two"></iframe>`,
			wantErr: errMultipleIframes,
		},
		{
			name:    "non-https",
			html:    `<iframe src="http://www.youtube.com/embed/one"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
		{
			name:    "userinfo",
			html:    `<iframe src="https://user@www.youtube.com/embed/one"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
		{
			name:    "ip host",
			html:    `<iframe src="https://127.0.0.1/embed/one"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
		{
			name:    "localhost",
			html:    `<iframe src="https://localhost/embed/one"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
		{
			name:    "disallowed host",
			html:    `<iframe src="https://example.com/embed/one"></iframe>`,
			wantErr: errEmbedHostDenied,
		},
		{
			name:    "allowed host with non-embed path",
			html:    `<iframe src="https://www.youtube.com/watch?v=MESYWwg-98I"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
		{
			name:    "allowed host with path traversal",
			html:    `<iframe src="https://www.youtube.com/embed/../watch?v=MESYWwg-98I"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
		{
			name:    "allowed host with encoded path traversal",
			html:    `<iframe src="https://www.youtube.com/embed/%2e%2e/watch?v=MESYWwg-98I"></iframe>`,
			wantErr: errInvalidIframeSrc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractOembedIframeURL(tt.html, "https://www.youtube.com/oembed", defaultEmbedHostAllowlist)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessURLDiscoversOembedForOpenGraphVideo(t *testing.T) {
	oembedHTML := `<div><iframe src="https://supercut.ai/embed/doist/Q5"></iframe></div>`
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodHead {
				return testHTTPResponse(req, http.StatusNotFound, "text/plain", ""), nil
			}
			switch req.URL.Host + req.URL.Path {
			case "supercut.ai/share/doist/Q5":
				return testHTTPResponse(req, http.StatusOK, "text/html", `<!doctype html>
					<html>
						<head>
							<meta property="og:title" content="Quoting Demo Review">
							<meta property="og:type" content="video.other">
							<meta property="og:image" content="https://meta.supercut.ai/thumb.png">
							<link rel="alternate" type="application/json+oembed" href="https://supercut.ai/oembed?url=https%3A%2F%2Fsupercut.ai%2Fshare%2Fdoist%2FQ5">
						</head>
						<body></body>
					</html>`), nil
			case "supercut.ai/oembed":
				return testHTTPResponse(req, http.StatusOK, "application/json", mustOembedJSON(t, map[string]any{
					"type":          "video",
					"title":         "Quoting Demo Review",
					"provider_name": "Supercut",
					"width":         3652,
					"height":        2132,
					"html":          oembedHTML,
				})), nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}
		}),
	}
	h := New(WithHTTPClient(client)).(*unfurlHandler)
	res := h.processURL(context.Background(), "https://supercut.ai/share/doist/Q5")
	if res.Title != "Quoting Demo Review" {
		t.Fatalf("unexpected title: %q", res.Title)
	}
	if res.EmbedURL != "https://supercut.ai/embed/doist/Q5" {
		t.Fatalf("unexpected EmbedURL: %q", res.EmbedURL)
	}
	if res.ProviderName != "Supercut" {
		t.Fatalf("unexpected ProviderName: %q", res.ProviderName)
	}
	if res.HTML != "" {
		t.Fatalf("oEmbed enrichment should not add raw HTML to OG video cards: %q", res.HTML)
	}
}

func TestProcessURLSkipsDisallowedOembedIframeWithoutDroppingUnfurl(t *testing.T) {
	oembedHTML := `<div><iframe src="https://example.com/embed/doist/Q5"></iframe></div>`
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodHead {
				return testHTTPResponse(req, http.StatusNotFound, "text/plain", ""), nil
			}
			switch req.URL.Host + req.URL.Path {
			case "supercut.ai/share/doist/Q5":
				return testHTTPResponse(req, http.StatusOK, "text/html", `<!doctype html>
					<html>
						<head>
							<meta property="og:title" content="Quoting Demo Review">
							<meta property="og:type" content="video.other">
							<meta property="og:image" content="https://meta.supercut.ai/thumb.png">
							<link rel="alternate" type="application/json+oembed" href="https://supercut.ai/oembed?url=https%3A%2F%2Fsupercut.ai%2Fshare%2Fdoist%2FQ5">
						</head>
						<body></body>
					</html>`), nil
			case "supercut.ai/oembed":
				return testHTTPResponse(req, http.StatusOK, "application/json", mustOembedJSON(t, map[string]any{
					"type":          "video",
					"title":         "Quoting Demo Review",
					"provider_name": "Supercut",
					"width":         3652,
					"height":        2132,
					"html":          oembedHTML,
				})), nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}
		}),
	}
	h := New(WithHTTPClient(client)).(*unfurlHandler)
	res := h.processURL(context.Background(), "https://supercut.ai/share/doist/Q5")
	if res.Title != "Quoting Demo Review" || res.Type != "video.other" || res.Image != "https://meta.supercut.ai/thumb.png" {
		t.Fatalf("unexpected unfurl result: %#v", res)
	}
	if res.EmbedURL != "" {
		t.Fatalf("disallowed iframe host should not produce EmbedURL, got %q", res.EmbedURL)
	}
	if res.HTML != "" {
		t.Fatalf("oEmbed enrichment should not add raw HTML to OG video cards: %q", res.HTML)
	}
}

func TestWithAllowedEmbedHostsKeepsDefaultsAndAddsCustomHosts(t *testing.T) {
	h := New(WithAllowedEmbedHosts([]string{"player.example.com"})).(*unfurlHandler)
	if !slices.Contains(h.embedHostAllowlist, "www.youtube.com") {
		t.Fatal("default YouTube embed host should remain allowed")
	}
	if !slices.Contains(h.embedHostAllowlist, "player.example.com") {
		t.Fatal("custom embed host should be allowed")
	}
}

func mustOembedJSON(t *testing.T, data map[string]any) string {
	t.Helper()
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func staticOembedResponse(body string) func(context.Context, string) (*http.Response, error) {
	return func(_ context.Context, endpointURL string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, endpointURL, nil)
		if err != nil {
			return nil, err
		}
		return testHTTPResponse(req, http.StatusOK, "application/json", body), nil
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testHTTPResponse(req *http.Request, status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
