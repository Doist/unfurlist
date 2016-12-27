package unfurlist

import (
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

// WithBlacklistPrefixes configures unfurl handler to skip unfurling urls
// matching any provided prefix
func WithBlacklistPrefixes(prefixes []string) ConfFunc {
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

// WithBlacklistTitles configures unfurl handler to skip unfurling urls that
// return pages which title contains one of substrings provided
func WithBlacklistTitles(substrings []string) ConfFunc {
	ss := make([]string, len(substrings))
	for i, s := range substrings {
		ss[i] = strings.ToLower(s)
	}
	return func(h *unfurlHandler) *unfurlHandler {
		if len(ss) > 0 {
			h.titleBlacklist = ss
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
