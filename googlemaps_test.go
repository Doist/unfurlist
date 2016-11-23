package unfurlist

import (
	"net/url"
	"testing"
)

func TestCoordsFromPath(t *testing.T) {
	testCases := []struct {
		input  string
		name   string
		coords string
		zoom   string
		ok     bool
	}{
		{"https://maps.google.com/maps/place/The+Manufacturing+Technology+Centre+(MTC)/@52.430763,-1.403385,16z/data=foo+bar",
			"The Manufacturing Technology Centre (MTC)",
			"52.430763,-1.403385", "16", true},
		{"https://www.google.com/maps/place/36%C2%B005'06.7%22N+5%C2%B030'49.6%22W/@36.0856728,-5.5169964,16z/data=",
			`36°05'06.7"N 5°30'49.6"W`, "36.0856728,-5.5169964", "16", true},
	}
	for _, tc := range testCases {
		u, err := url.Parse(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		name, coords, zoom, ok := coordsFromPath(u.Path)
		if ok != tc.ok || name != tc.name || coords != tc.coords || zoom != tc.zoom {
			t.Fatalf("wrong result for input %q: got name:%q, coords:%q, zoom:%q, ok:%v", tc.input, name, coords, zoom, ok)
		}
	}
}
