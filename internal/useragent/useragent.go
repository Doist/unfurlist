// This is a vendored copy of https://github.com/artyom/useragent
//
// Package useragent provides http.RoundTripper wrapper to set User-Agent header
// on each http request made.
//
// Basic usage:
//
// 	client := &http.Client{
// 		Transport: useragent.Set(http.DefaultTransport, "MyRobot/1.0"),
// 	}
// 	resp, err := client.Get("https://...")
package useragent

import (
	"net/http"
	"strings"
)

// Set wraps provided http.RoundTripper returning a new one that adds given
// agent as User-Agent header for requests without such header or with empty
// User-Agent header.
//
// If rt is a *http.Transport, the returned RoundTripper would have Transport's
// methods visible so they can be accessed after type assertion to required
// interface.
func Set(rt http.RoundTripper, agent string) http.RoundTripper {
	if agent == "" {
		return rt
	}
	if t, ok := rt.(*http.Transport); ok {
		return uaT{t, agent}
	}
	return uaRT{rt, agent}
}

type uaT struct {
	*http.Transport
	userAgent string
}

func (t uaT) RoundTrip(r *http.Request) (*http.Response, error) {
	if _, ok := r.Header["User-Agent"]; ok {
		return t.Transport.RoundTrip(r)
	}
	r2 := new(http.Request)
	*r2 = *r
	r2.Header = make(http.Header, len(r.Header)+1)
	for k, v := range r.Header {
		r2.Header[k] = v
	}
	r2.Header.Set("User-Agent", t.userAgent)
	if r.URL.Host == "twitter.com" || strings.HasSuffix(r.URL.Host, ".twitter.com") {
		r2.Header.Set("User-Agent", "DiscourseBot/1.0")
	}
	return t.Transport.RoundTrip(r2)
}

type uaRT struct {
	http.RoundTripper
	userAgent string
}

func (t uaRT) RoundTrip(r *http.Request) (*http.Response, error) {
	if _, ok := r.Header["User-Agent"]; ok {
		return t.RoundTripper.RoundTrip(r)
	}
	r2 := new(http.Request)
	*r2 = *r
	r2.Header = make(http.Header, len(r.Header)+1)
	for k, v := range r.Header {
		r2.Header[k] = v
	}
	r2.Header.Set("User-Agent", t.userAgent)
	return t.RoundTripper.RoundTrip(r2)
}
