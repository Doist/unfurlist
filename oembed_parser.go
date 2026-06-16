package unfurlist

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/artyom/oembed"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/idna"
)

var defaultEmbedHostAllowlist = []string{
	"supercut.ai",
	"www.loom.com",
	"www.youtube.com",
	"www.youtube-nocookie.com",
	"player.vimeo.com",
}

var defaultEmbedPathPrefixes = map[string][]string{
	"player.vimeo.com":         {"/video/"},
	"supercut.ai":              {"/embed/"},
	"www.loom.com":             {"/embed/"},
	"www.youtube.com":          {"/embed/"},
	"www.youtube-nocookie.com": {"/embed/"},
}

func fetchOembed(ctx context.Context, endpointURL string, fn func(context.Context, string) (*http.Response, error), embedHostAllowlist []string) (*unfurlResult, error) {
	resp, err := fn(ctx, endpointURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	baseURL := endpointURL
	if resp.Request != nil && resp.Request.URL != nil {
		baseURL = resp.Request.URL.String()
	}
	meta, err := oembed.FromResponse(resp)
	if err != nil {
		return nil, err
	}
	res := &unfurlResult{
		Title:        meta.Title,
		SiteName:     meta.Provider,
		ProviderName: meta.Provider,
		Type:         string(meta.Type),
		HTML:         meta.HTML,
		Image:        meta.Thumbnail,
		ImageWidth:   meta.ThumbnailWidth,
		ImageHeight:  meta.ThumbnailHeight,
	}
	if meta.Type == oembed.TypePhoto && meta.URL != "" {
		res.Image = meta.URL
		res.ImageWidth = meta.Width
		res.ImageHeight = meta.Height
	}
	if meta.Type == oembed.TypeVideo && meta.HTML != "" {
		if embedURL, err := extractOembedIframeURL(meta.HTML, baseURL, embedHostAllowlist); err == nil {
			res.EmbedURL = embedURL
			res.EmbedWidth = meta.Width
			res.EmbedHeight = meta.Height
		}
	}
	return res, nil
}

var (
	errNoIframe         = errors.New("oembed html has no iframe")
	errMultipleIframes  = errors.New("oembed html has multiple iframes")
	errUnsafeEmbedHTML  = errors.New("oembed html contains unsafe embed elements")
	errInvalidIframeSrc = errors.New("oembed iframe src is invalid")
	errEmbedHostDenied  = errors.New("oembed iframe host is not allowed")
)

func extractOembedIframeURL(snippet, baseURL string, embedHostAllowlist []string) (string, error) {
	z := html.NewTokenizer(strings.NewReader(snippet))
	var iframeSrc string
	for {
		switch tt := z.Next(); tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				goto finish
			}
			return "", z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Script, atom.Object, atom.Embed:
				return "", errUnsafeEmbedHTML
			case atom.Iframe:
				if iframeSrc != "" {
					return "", errMultipleIframes
				}
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					if strings.EqualFold(string(k), "src") {
						iframeSrc = string(v)
					}
				}
			}
		}
	}

finish:
	if iframeSrc == "" {
		return "", errNoIframe
	}
	return validateEmbedURL(iframeSrc, baseURL, embedHostAllowlist)
}

func validateEmbedURL(rawURL, baseURL string, embedHostAllowlist []string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errInvalidIframeSrc
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", errInvalidIframeSrc
	}
	if !u.IsAbs() {
		base, err := url.Parse(baseURL)
		if err != nil || base.Scheme == "" || base.Host == "" {
			return "", errInvalidIframeSrc
		}
		u = base.ResolveReference(u)
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil || u.Port() != "" {
		return "", errInvalidIframeSrc
	}
	if pathHasDotSegment(u.EscapedPath()) {
		return "", errInvalidIframeSrc
	}
	host, err := normalizeEmbedHost(u.Hostname())
	if err != nil || host == "" {
		return "", errInvalidIframeSrc
	}
	if ip := net.ParseIP(host); ip != nil {
		return "", errInvalidIframeSrc
	}
	u.Host = host
	if !slices.Contains(embedHostAllowlist, host) {
		return "", errEmbedHostDenied
	}
	if !embedPathAllowed(host, u.Path) {
		return "", errInvalidIframeSrc
	}
	return u.String(), nil
}

func pathHasDotSegment(path string) bool {
	path, err := url.PathUnescape(path)
	if err != nil {
		return true
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

func embedPathAllowed(host, path string) bool {
	prefixes, ok := defaultEmbedPathPrefixes[host]
	if !ok {
		return true
	}
	return slices.ContainsFunc(prefixes, func(prefix string) bool {
		return strings.HasPrefix(path, prefix)
	})
}

func appendUniqueEmbedHosts(hosts []string, values ...string) []string {
	for _, value := range values {
		normalized, err := normalizeEmbedHost(value)
		if err != nil {
			continue
		}
		if !slices.Contains(hosts, normalized) {
			hosts = append(hosts, normalized)
		}
	}
	return hosts
}

func normalizeEmbedHost(host string) (string, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return "", errInvalidIframeSrc
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(host, ".localhost") {
		return "", errInvalidIframeSrc
	}
	return idna.Lookup.ToASCII(host)
}
