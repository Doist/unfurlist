package unfurlist

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// instagramFetcher retrieves metadata for Instagram post/reel urls from
// Instagram's public embed endpoint. This is needed because instagram.com
// serves the main page without any usable metadata (or redirects to a login
// page) to most non-browser clients, while the embed endpoint — the one
// backing embedded posts on third-party sites — is served with post details.
func instagramFetcher(ctx context.Context, fn func(context.Context, string) (*http.Response, error), link string) (*Metadata, bool) {
	embedURL, ok := instagramEmbedURL(link)
	if !ok {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := fn(ctx, embedURL)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, false
	}
	if meta, ok := instagramParseContextJSON(body); ok {
		return meta, true
	}
	return instagramParseEmbedHTML(body)
}

var instagramPostRe = regexp.MustCompile(`^(?:/[A-Za-z0-9._]+)?/(p|reel|reels|tv)/([A-Za-z0-9_-]+)/?$`)

// instagramEmbedURL maps an Instagram post/reel/tv url to its public embed
// endpoint; ok is false if link is not such an url.
func instagramEmbedURL(link string) (string, bool) {
	u, err := url.Parse(link)
	if err != nil {
		return "", false
	}
	switch strings.TrimPrefix(strings.ToLower(u.Host), "www.") {
	case "instagram.com", "instagr.am":
	default:
		return "", false
	}
	m := instagramPostRe.FindStringSubmatch(u.Path)
	if m == nil {
		return "", false
	}
	kind := m[1]
	if kind == "reels" {
		kind = "reel"
	}
	return "https://www.instagram.com/" + kind + "/" + m[2] + "/embed/captioned/", true
}

// instagramParseContextJSON extracts metadata from the "contextJSON" payload
// that Instagram embeds into a script on embed pages of video posts (reels).
func instagramParseContextJSON(body []byte) (*Metadata, bool) {
	_, after, found := bytes.Cut(body, []byte(`"contextJSON":`))
	if !found {
		return nil, false
	}
	var contextJSON string
	if err := json.NewDecoder(bytes.NewReader(after)).Decode(&contextJSON); err != nil {
		return nil, false
	}
	var payload struct {
		GqlData struct {
			Media *struct {
				IsVideo          bool   `json:"is_video"`
				DisplayURL       string `json:"display_url"`
				DisplayResources []struct {
					Src    string `json:"src"`
					Width  int    `json:"config_width"`
					Height int    `json:"config_height"`
				} `json:"display_resources"`
				Caption struct {
					Edges []struct {
						Node struct {
							Text string `json:"text"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"edge_media_to_caption"`
				Owner struct {
					Username string `json:"username"`
				} `json:"owner"`
			} `json:"shortcode_media"`
		} `json:"gql_data"`
	}
	if err := json.Unmarshal([]byte(contextJSON), &payload); err != nil {
		return nil, false
	}
	media := payload.GqlData.Media
	if media == nil {
		return nil, false
	}
	var caption string
	if len(media.Caption.Edges) > 0 {
		caption = media.Caption.Edges[0].Node.Text
	}
	meta := &Metadata{
		Title:       instagramTitle(media.Owner.Username, caption),
		Type:        "article",
		Description: caption,
		Image:       media.DisplayURL,
	}
	if media.IsVideo {
		meta.Type = "video.other"
	}
	for _, r := range media.DisplayResources {
		if r.Src != "" && r.Width*r.Height >= meta.ImageWidth*meta.ImageHeight {
			meta.Image, meta.ImageWidth, meta.ImageHeight = r.Src, r.Width, r.Height
		}
	}
	if !meta.Valid() {
		return nil, false
	}
	return meta, true
}

// instagramParseEmbedHTML extracts metadata from static markup of embed pages;
// Instagram uses it instead of the "contextJSON" payload for photo posts.
func instagramParseEmbedHTML(body []byte) (*Metadata, bool) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, false
	}
	var image, username, caption string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.DataAtom == atom.Img && hasClass(n, "EmbeddedMediaImage"):
				if image == "" {
					image = attrValue(n, "src")
				}
			case hasClass(n, "UsernameText"):
				if username == "" {
					username = nodeText(n)
				}
			case hasClass(n, "Caption"):
				if caption == "" {
					caption = captionText(n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	meta := &Metadata{
		Title:       instagramTitle(username, caption),
		Type:        "article",
		Description: caption,
		Image:       image,
	}
	if !meta.Valid() {
		return nil, false
	}
	return meta, true
}

// instagramTitle mimics the og:title format that instagram.com serves to
// allowed clients: `username on Instagram: "caption"`
func instagramTitle(username, caption string) string {
	if username == "" {
		return ""
	}
	title := username + " on Instagram"
	if caption = strings.Join(strings.Fields(caption), " "); caption != "" {
		if utf8.RuneCountInString(caption) > 100 {
			caption = string([]rune(caption)[:100]) + "…"
		}
		title += `: "` + caption + `"`
	}
	return title
}

// captionText returns text content of the Caption element of an embed page,
// skipping the leading username link and the trailing comments block.
func captionText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (hasClass(n, "CaptionUsername") || hasClass(n, "CaptionComments")) {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Br {
			b.WriteString("\n")
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return strings.TrimSpace(b.String())
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func hasClass(n *html.Node, name string) bool {
	return slices.Contains(strings.Fields(attrValue(n, "class")), name)
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
