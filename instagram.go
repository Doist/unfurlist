package unfurlist

import (
	"net/url"
	"strings"
)

// canonicalInstagramOembedURL normalizes supported Instagram post URLs to the
// forms accepted by Meta's oEmbed endpoint. The original URL is still returned
// to the caller in the unfurl result.
func canonicalInstagramOembedURL(link string) string {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Port() != "" {
		return link
	}
	switch strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") {
	case "instagram.com", "instagr.am":
	default:
		return link
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var kind, shortcode string
	switch {
	case len(parts) >= 2 && isInstagramMediaKind(parts[0]):
		kind, shortcode = parts[0], parts[1]
	case len(parts) == 2 && parts[0] == "reels":
		kind, shortcode = "reel", parts[1]
	case len(parts) >= 3 && isInstagramMediaKind(parts[1]):
		kind, shortcode = parts[1], parts[2]
	default:
		return link
	}
	if shortcode == "" {
		return link
	}
	u.Path = "/" + kind + "/" + shortcode + "/"
	u.RawPath = ""
	return u.String()
}

func isInstagramMediaKind(kind string) bool {
	return kind == "p" || kind == "reel" || kind == "tv"
}
