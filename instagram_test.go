package unfurlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestInstagramEmbedURL(t *testing.T) {
	testCases := []struct {
		link string
		want string // empty means no match expected
	}{
		{"https://www.instagram.com/reel/DaVYQZlER5B/?igsh=a3N5cG0yY3IzMHc5",
			"https://www.instagram.com/reel/DaVYQZlER5B/embed/captioned/"},
		{"https://www.instagram.com/reels/DaVYQZlER5B/",
			"https://www.instagram.com/reel/DaVYQZlER5B/embed/captioned/"},
		{"https://www.instagram.com/tntsportsbr/reel/DaVYQZlER5B/",
			"https://www.instagram.com/reel/DaVYQZlER5B/embed/captioned/"},
		{"https://instagram.com/p/BsOGulcndj-",
			"https://www.instagram.com/p/BsOGulcndj-/embed/captioned/"},
		{"https://instagr.am/p/BsOGulcndj-/",
			"https://www.instagram.com/p/BsOGulcndj-/embed/captioned/"},
		{"https://www.instagram.com/tv/CDcZ5wYJdXd/",
			"https://www.instagram.com/tv/CDcZ5wYJdXd/embed/captioned/"},
		{"https://www.instagram.com/tntsportsbr/", ""},
		{"https://www.instagram.com/reels/audio/892839493/", ""},
		{"https://www.instagram.com/stories/tntsportsbr/123/", ""},
		{"https://www.instagram.com/accounts/login/", ""},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", ""},
		{"https://notinstagram.com/reel/DaVYQZlER5B/", ""},
	}
	for _, tc := range testCases {
		got, ok := instagramEmbedURL(tc.link)
		if ok != (tc.want != "") || got != tc.want {
			t.Errorf("instagramEmbedURL(%q) = %q, %v; want %q", tc.link, got, ok, tc.want)
		}
	}
}

func TestInstagramFetcher(t *testing.T) {
	serve := func(status int, bodyFile string) func(context.Context, string) (*http.Response, error) {
		return func(_ context.Context, url string) (*http.Response, error) {
			const wantURL = "https://www.instagram.com/reel/DaVYQZlER5B/embed/captioned/"
			if url != wantURL {
				t.Errorf("fetch of %q; want %q", url, wantURL)
			}
			var body []byte
			if bodyFile != "" {
				var err error
				if body, err = os.ReadFile(bodyFile); err != nil {
					t.Fatal(err)
				}
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
	}
	const link = "https://www.instagram.com/reel/DaVYQZlER5B/"
	if meta, ok := instagramFetcher(t.Context(), serve(http.StatusOK, "testdata/instagram-reel-embed"), link); !ok || meta.Type != "video.other" {
		t.Errorf("reel embed page: got %+v, %v", meta, ok)
	}
	if meta, ok := instagramFetcher(t.Context(), serve(http.StatusOK, "testdata/instagram-photo-embed"), link); !ok || meta.Type != "article" {
		t.Errorf("photo embed page: got %+v, %v", meta, ok)
	}
	if meta, ok := instagramFetcher(t.Context(), serve(http.StatusTooManyRequests, "testdata/instagram-reel-embed"), link); ok {
		t.Errorf("non-200 status: unexpected metadata %+v", meta)
	}
	failFn := func(context.Context, string) (*http.Response, error) { return nil, errors.New("boom") }
	if meta, ok := instagramFetcher(t.Context(), failFn, link); ok {
		t.Errorf("fetch error: unexpected metadata %+v", meta)
	}
	noMatchFn := func(context.Context, string) (*http.Response, error) {
		t.Error("unexpected fetch for non-post url")
		return nil, errors.New("unreachable")
	}
	if meta, ok := instagramFetcher(t.Context(), noMatchFn, "https://www.instagram.com/tntsportsbr/"); ok {
		t.Errorf("non-post url: unexpected metadata %+v", meta)
	}
}

func TestInstagramParseContextJSON(t *testing.T) {
	body, err := os.ReadFile("testdata/instagram-reel-embed")
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := instagramParseContextJSON(body)
	if !ok {
		t.Fatal("no metadata parsed from reel embed page")
	}
	want := Metadata{
		Title:       `tntsportsbr on Instagram: "O MUNDO ESTÁ CABO-VERDEADO!!! 🇨🇻🇨🇻🇨🇻 #TNTSports2026"`,
		Type:        "video.other",
		Description: "O MUNDO ESTÁ CABO-VERDEADO!!! 🇨🇻🇨🇻🇨🇻 #TNTSports2026",
	}
	if meta.Title != want.Title || meta.Type != want.Type || meta.Description != want.Description {
		t.Errorf("got %+v\nwant title %q, type %q, description %q", meta, want.Title, want.Type, want.Description)
	}
	if !strings.Contains(meta.Image, "jpg") || meta.ImageWidth == 0 || meta.ImageHeight == 0 {
		t.Errorf("bad image data: url %q, %dx%d", meta.Image, meta.ImageWidth, meta.ImageHeight)
	}
}

func TestInstagramParseContextJSON_edgeCases(t *testing.T) {
	page := func(contextJSON string) []byte {
		b, err := json.Marshal(contextJSON)
		if err != nil {
			t.Fatal(err)
		}
		return []byte(`<html><body><script>init([{"contextJSON":` + string(b) + `}])</script></body></html>`)
	}
	t.Run("no caption, no display_resources", func(t *testing.T) {
		body := page(`{"gql_data":{"shortcode_media":{"is_video":false,"display_url":"https://example.com/pic.jpg","owner":{"username":"someuser"}}}}`)
		meta, ok := instagramParseContextJSON(body)
		if !ok {
			t.Fatal("no metadata parsed")
		}
		if meta.Title != "someuser on Instagram" || meta.Type != "article" ||
			meta.Image != "https://example.com/pic.jpg" || meta.ImageWidth != 0 {
			t.Errorf("got %+v", meta)
		}
	})
	t.Run("no media", func(t *testing.T) {
		if meta, ok := instagramParseContextJSON(page(`{"gql_data":null}`)); ok {
			t.Errorf("unexpected metadata: %+v", meta)
		}
	})
	t.Run("media without usable fields", func(t *testing.T) {
		if meta, ok := instagramParseContextJSON(page(`{"gql_data":{"shortcode_media":{"is_video":true}}}`)); ok {
			t.Errorf("unexpected metadata: %+v", meta)
		}
	})
	t.Run("garbage", func(t *testing.T) {
		for _, body := range []string{"", "<html></html>", `{"contextJSON":12`, `..."contextJSON":null...`} {
			if meta, ok := instagramParseContextJSON([]byte(body)); ok {
				t.Errorf("unexpected metadata from %q: %+v", body, meta)
			}
			if meta, ok := instagramParseEmbedHTML([]byte(body)); ok {
				t.Errorf("unexpected metadata from %q: %+v", body, meta)
			}
		}
	})
}

func TestInstagramParseEmbedHTML(t *testing.T) {
	body, err := os.ReadFile("testdata/instagram-photo-embed")
	if err != nil {
		t.Fatal(err)
	}
	// photo embed pages carry "contextJSON":null, the JSON parse must
	// fall through to markup parsing just like instagramFetcher does
	if meta, ok := instagramParseContextJSON(body); ok {
		t.Fatalf("unexpected contextJSON metadata from photo embed page: %+v", meta)
	}
	meta, ok := instagramParseEmbedHTML(body)
	if !ok {
		t.Fatal("no metadata parsed from photo embed page")
	}
	const wantTitle = `world_record_egg on Instagram: "Let’s set a world record together and get the most liked post on Instagram. Beating the current worl…"`
	if meta.Title != wantTitle {
		t.Errorf("title = %q; want %q", meta.Title, wantTitle)
	}
	if !strings.HasPrefix(meta.Description, "Let’s set a world record together") ||
		!strings.Contains(meta.Description, "#LikeTheEgg") {
		t.Errorf("bad description: %q", meta.Description)
	}
	if meta.Type != "article" {
		t.Errorf("type = %q; want article", meta.Type)
	}
	if !strings.Contains(meta.Image, "jpg") {
		t.Errorf("bad image url: %q", meta.Image)
	}
}
