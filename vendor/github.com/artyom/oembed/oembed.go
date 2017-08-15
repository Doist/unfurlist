// Package oembed implements functions to discover oEmbed providers for given
// URL and parse providers' response.
package oembed

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gobwas/glob"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Type describes resource type as returned by oEmbed provider
type Type string

// Predefined oEmbed types (http://oembed.com 2.3.4.1-2.3.4.4):
const (
	TypePhoto Type = "photo"
	TypeVideo      = "video"
	TypeLink       = "link"
	TypeRich       = "rich"
)

// Metadata describes resource as returned by oEmbed provider
// (http://oembed.com, section 2.3.4)
type Metadata struct {
	Type            Type   // resource type
	Provider        string // provider name
	Title           string // resource title
	AuthorName      string // name of the author/owner
	AuthorURL       string // URL for the author/owner of the resource
	Thumbnail       string // URL of the thumbnail image
	ThumbnailWidth  int    // width of the optional thumbnail
	ThumbnailHeight int    // height of the optional thumbnail

	// HTML snippet required to display the embedded resource — as returned
	// by provider, may be unsafe. Set for video and rich types.
	HTML          string
	URL           string // source URL of the original image, set for photo types
	Width, Height int    // width/height in pixels of the image/snippet specified by the URL/HTML
}

type decoder interface {
	Decode(interface{}) error
}

func decode(dec decoder) (*Metadata, error) {
	meta := struct {
		XMLName         xml.Name `json:"-"  xml:"oembed"`
		Provider        string   `json:"provider_name" xml:"provider_name"`
		Type            string   `json:"type" xml:"type"`
		Title           string   `json:"title" xml:"title"`
		AuthorName      string   `json:"author_name" xml:"author_name"`
		AuthorURL       string   `json:"author_url" xml:"author_url"`
		Thumbnail       string   `json:"thumbnail_url" xml:"thumbnail_url"`
		ThumbnailWidth  int      `json:"thumbnail_width" xml:"thumbnail_width"`
		ThumbnailHeight int      `json:"thumbnail_height" xml:"thumbnail_height"`
		HTML            string   `json:"html" xml:"html"`
		URL             string   `json:"url" xml:"url"`
		Width           int      `json:"width" xml:"width"`
		Height          int      `json:"height" xml:"height"`
	}{}
	if err := dec.Decode(&meta); err != nil {
		return nil, err
	}
	m := &Metadata{
		Type:       Type(strings.ToLower(meta.Type)),
		Provider:   meta.Provider,
		Title:      meta.Title,
		AuthorName: meta.AuthorName,
		AuthorURL:  meta.AuthorURL,
		Thumbnail:  meta.Thumbnail,
	}
	switch m.Type {
	case TypePhoto:
		if u := meta.URL; urlSupported(u) {
			m.URL = u
		}
		if m.URL != "" && meta.Width > 0 && meta.Height > 0 {
			m.Width, m.Height = meta.Width, meta.Height
		}
	case TypeVideo, TypeRich:
		m.HTML = meta.HTML
		if m.HTML != "" && meta.Width > 0 && meta.Height > 0 {
			m.Width, m.Height = meta.Width, meta.Height
		}
	case TypeLink:
	default:
		return nil, fmt.Errorf("oembed: unsupported result type: %q", m.Type)
	}
	if !urlSupported(m.Thumbnail) {
		m.Thumbnail = ""
	}
	if m.Thumbnail != "" && meta.ThumbnailWidth > 0 && meta.ThumbnailHeight > 0 {
		m.ThumbnailWidth = meta.ThumbnailWidth
		m.ThumbnailHeight = meta.ThumbnailHeight
	}
	if !urlSupported(m.AuthorURL) {
		m.AuthorURL = ""
	}
	return m, nil
}

func urlSupported(u string) bool {
	_u, err := url.Parse(u)
	if err != nil {
		return false
	}
	switch _u.Scheme {
	case "http", "https":
		return _u.Host != ""
	}
	return false
}

// FromJSON decodes json-encoded oEmbed metadata read from r. It ensures that
// result contains only valid http/https urls and metadata satisfies one of the
// predefined types.
func FromJSON(r io.Reader) (*Metadata, error) { return decode(json.NewDecoder(r)) }

// FromXML decodes xml-encoded oEmbed metadata read from r. It ensures that
// result contains only valid http/https urls and metadata satisfies one of the
// predefined types.
func FromXML(r io.Reader) (*Metadata, error) { return decode(xml.NewDecoder(r)) }

// FromResponse verifies status code of r, and calls FromJSON
// or FromXML depending on Content-Type of r. Only responses with status code
// 200 and Content-Type of either application/json or text/xml are allowed,
// otherwise error is returned.
func FromResponse(r *http.Response) (*Metadata, error) {
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oembed: unsupported response status: %q", r.Status)
	}
	switch ct := strings.ToLower(r.Header.Get("Content-Type")); {
	case ct == "application/json" || strings.HasPrefix(ct, "application/json;"):
		return FromJSON(r.Body)
	case ct == "text/xml" || strings.HasPrefix(ct, "text/xml;"):
		return FromXML(r.Body)
	}
	return nil, fmt.Errorf("oembed: unsupported Content-Type: %q", r.Header.Get("Content-Type"))
}

// LookupFunc checks whether provided url matches any known oEmbed provider and
// returns url of oEmbed endpoint that should return metadata about original
// url.
type LookupFunc func(url string) (endpointURL string, found bool)

// Discover parses htmlBodyReader as utf-8 encoded html text and tries to
// discover oEmbed endpoint as described in http://oembed.com/#section4
//
// Use NewReader from golang.org/x/net/html/charset to ensure data is properly
// encoded.
func Discover(htmlBodyReader io.Reader) (endpointURL string, found bool, err error) {
	z := html.NewTokenizer(htmlBodyReader)
tokenize:
	for {
		switch tt := z.Next(); tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return "", false, nil
			}
			return "", false, z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				return "", false, nil
			case atom.Link:
				var link string
				var isOembedLink bool
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "type":
						switch {
						case bytes.Equal(v, []byte("application/json+oembed")):
						case bytes.Equal(v, []byte("text/xml+oembed")):
						default:
							continue tokenize
						}
						isOembedLink = true
					case "href":
						link = string(v)
					}
				}
				if isOembedLink && link != "" {
					return link, true, nil
				}
			}
		}
	}
}

type endpoint struct {
	https  bool
	domain glob.Glob
	path   glob.Glob

	u *url.URL
}

type endpoints []endpoint

func (providers endpoints) lookup(urlString string) (endpointURL string, found bool) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", false
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return "", false
	}
	for _, ep := range providers {
		if u.Scheme == "https" != ep.https ||
			// *.domain.com should also match domain.com
			!(ep.domain.Match(u.Host) || ep.domain.Match("."+u.Host)) ||
			!ep.path.Match(u.Path) {
			continue
		}
		u2 := new(url.URL)
		*u2 = *ep.u
		vals := u2.Query()
		vals["url"] = []string{urlString}
		u2.RawQuery = vals.Encode()
		return u2.String(), true
	}
	return "", false
}

// Providers decodes r as content of http://oembed.com/providers.json and
// returns LookupFunc matching url against providers list.
func Providers(r io.Reader) (LookupFunc, error) {
	ps := []struct {
		Endpoints []struct {
			URL     string   `json:"url"`
			Schemes []string `json:"schemes,omitempty"`
		} `json:"endpoints"`
	}{}
	if err := json.NewDecoder(r).Decode(&ps); err != nil {
		return nil, err
	}
	var providers endpoints
	for _, p := range ps {
		for _, ep := range p.Endpoints {
			if ep.URL == "" || len(ep.Schemes) == 0 {
				continue
			}
			u, err := url.Parse(strings.Replace(ep.URL, "{format}", "json", -1))
			if err != nil {
				continue
			}
			for _, pat := range ep.Schemes {
				u2, err := url.Parse(pat)
				if err != nil {
					continue
				}
				switch u2.Scheme {
				case "http", "https":
				default:
					continue
				}
				if idx := strings.IndexByte(u2.Host, '*'); idx != strings.LastIndexByte(u2.Host, '*') ||
					idx != strings.Index(u2.Host, "*.") ||
					(idx >= 0 && strings.Count(u2.Host[idx:], ".") < 2) {
					continue
				}
				endpoint := endpoint{u: u, https: u2.Scheme == "https"}
				if endpoint.domain, err = glob.Compile(u2.Host); err != nil {
					continue
				}
				if endpoint.path, err = glob.Compile(u2.Path); err != nil {
					continue
				}
				providers = append(providers, endpoint)
			}
		}
	}
	return providers.lookup, nil
}
