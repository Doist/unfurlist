package unfurlist

import (
	"crypto/sha1"
	"net/http"
	"strings"

	"github.com/bradfitz/gomemcache/memcache"
)

// WithHTTPClient configures unfurl handler to use provided http.Client for
// outgoing requests
func WithHTTPClient(client *http.Client) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		if client != nil {
			h.HTTPClient = client
		}
		return h
	}
}

// WithMemcache configures unfurl handler to cache metadata in memcached
func WithMemcache(client *memcache.Client) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		if client != nil {
			h.Cache = client
		}
		return h
	}
}

// WithExtraHeaders configures unfurl handler to add extra headers to each
// outgoing http request
func WithExtraHeaders(hdr map[string]string) ConfFunc {
	headers := make([]string, 0, len(hdr)*2)
	for k, v := range hdr {
		headers = append(headers, k, v)
	}
	return func(h *unfurlHandler) *unfurlHandler {
		h.Headers = headers
		return h
	}
}

// WithBlocklistPrefixes configures unfurl handler to skip unfurling urls
// matching any provided prefix
func WithBlocklistPrefixes(prefixes []string) ConfFunc {
	var pmap *prefixMap
	if len(prefixes) > 0 {
		pmap = newPrefixMap(prefixes)
	}
	return func(h *unfurlHandler) *unfurlHandler {
		if pmap != nil {
			h.pmap = pmap
		}
		return h
	}
}

// WithBlocklistTitles configures unfurl handler to skip unfurling urls that
// return pages which title contains one of substrings provided
func WithBlocklistTitles(substrings []string) ConfFunc {
	ss := make([]string, len(substrings))
	for i, s := range substrings {
		ss[i] = strings.ToLower(s)
	}
	return func(h *unfurlHandler) *unfurlHandler {
		if len(ss) > 0 {
			h.titleBlocklist = ss
		}
		return h
	}
}

// WithImageDimensions configures unfurl handler whether to fetch image
// dimensions or not.
func WithImageDimensions(enable bool) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		h.FetchImageSize = enable
		return h
	}
}

// WithFetchers attaches custom fetchers to unfurl handler created by New().
func WithFetchers(fetchers ...FetchFunc) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		h.fetchers = fetchers
		return h
	}
}

// WithImageProxy configures unfurl handler to pass plain http image urls
// through image proxy located at proxyURL. The following query parameters are
// added to the proxyURL: "u" specifies original image url, "h" specifies sha1
// HMAC signature (only if secret is not empty). It is expected that proxyURL
// does not have query string; it is used "as is", query arguments are appended
// as "?u=...&h=..." string.
//
// See https://github.com/artyom/image-proxy for proxy implementation example.
func WithImageProxy(proxyURL, secret string) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		h.imageProxyURL = proxyURL
		if secret != "" {
			b := sha1.Sum([]byte(secret))
			h.imageProxyKey = b[:]
		}
		return h
	}
}

// WithMaxResults configures unfurl handler to only process n first urls it
// finds. n must be positive.
func WithMaxResults(n int) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		if n > 0 {
			h.maxResults = n
		}
		return h
	}
}

// WithLogger configures unfurl handler to use provided logger
func WithLogger(l Logger) ConfFunc {
	return func(h *unfurlHandler) *unfurlHandler {
		if l != nil {
			h.Log = l
		}
		return h
	}
}

// Logger describes set of methods used by unfurl handler for logging; standard
// lib *log.Logger implements this interface.
type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}
