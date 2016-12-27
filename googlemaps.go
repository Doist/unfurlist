package unfurlist

import (
	"net/url"
	"regexp"
	"strings"
)

// GoogleMapsFetcher returns FetchFunc that recognizes some Google Maps urls and
// constructs metadata for them containing preview image from Google Static Maps
// API. The only argument is the API key to create image links with.
func GoogleMapsFetcher(key string) FetchFunc {
	if key == "" {
		return func(*url.URL) (*Metadata, bool) { return nil, false }
	}
	return func(u *url.URL) (*Metadata, bool) {
		if u == nil {
			return nil, false
		}
		if idx := strings.LastIndexByte(u.Host, '.'); idx == -1 ||
			!(strings.HasSuffix(u.Host[:idx], ".google") &&
				strings.HasPrefix(u.Path, "/maps")) {
			return nil, false
		}
		if u.Path == "/maps/api/staticmap" {
			return &Metadata{Image: u.String(), Type: "image"}, true
		}
		g := &url.URL{
			Scheme: "https",
			Host:   "maps.googleapis.com",
			Path:   "/maps/api/staticmap",
		}
		vals := make(url.Values)
		vals.Set("key", key)
		vals.Set("zoom", "16")
		vals.Set("size", "640x480")
		vals.Set("scale", "2")
		if q := u.Query().Get("q"); u.Path == "/maps" && q != "" {
			if zoom := u.Query().Get("z"); zoom != "" {
				vals.Set("zoom", zoom)
			}
			vals.Set("markers", "color:red|"+q)
			g.RawQuery = vals.Encode()
			return &Metadata{
				Type:        "website",
				Image:       g.String(),
				ImageWidth:  640 * 2,
				ImageHeight: 480 * 2,
			}, true
		}
		name, coords, zoom, ok := coordsFromPath(u.Path)
		if !ok {
			return &Metadata{Title: "Google Maps", Type: "website"}, true
		}
		vals.Set("zoom", zoom)
		vals.Set("markers", "color:red|"+coords)
		g.RawQuery = vals.Encode()
		return &Metadata{
			Title:       name,
			Type:        "website",
			Image:       g.String(),
			ImageWidth:  640 * 2,
			ImageHeight: 480 * 2,
		}, true
	}
}

var googlePlace = regexp.MustCompile(`^/maps/place/(?P<name>[^/]+)/@(?P<coords>[0-9.-]+,[0-9.-]+),(?P<zoom>[0-9.]+)z`)

// coordsFromPath extracts name, coordinates and zoom level from urls of the
// following format:
// https://www.google.com/maps/place/Passeig+de+Gràcia,+Barcelona,+Spain/@41.3931702,2.1617715,17z
func coordsFromPath(p string) (name, coords, zoom string, ok bool) {
	ix := googlePlace.FindStringSubmatchIndex(p)
	// 4*2 is len(googlePlace.SubexpNames())*2
	if ix == nil || len(ix) != 4*2 {
		return "", "", "", false
	}
	name = p[ix[1*2]:ix[1*2+1]]
	coords = p[ix[2*2]:ix[2*2+1]]
	zoom = p[ix[3*2]:ix[3*2+1]]
	// normally p is already unescaped URL.Path, but it still has spaces
	// presented as +, this unescapes them
	if name, err := url.QueryUnescape(name); err == nil {
		return name, coords, zoom, true
	}
	return name, coords, zoom, true
}
