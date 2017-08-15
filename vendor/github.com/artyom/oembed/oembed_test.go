package oembed

import (
	"io"
	"os"
	"strings"
	"testing"
)

func reference() *Metadata {
	return &Metadata{
		Type:            TypeRich,
		Provider:        "eduMedia",
		Title:           "Soil formation",
		AuthorName:      "eduMedia",
		AuthorURL:       "http://www.edumedia-sciences.com/en/",
		Thumbnail:       "http://www.edumedia-sciences.com/media/thumbnail/531",
		ThumbnailWidth:  100,
		ThumbnailHeight: 100,
		Width:           550,
		Height:          440,
		HTML:            `<iframe width=550 height=440 src="https://www.edumedia-sciences.com/en/media/frame/531/" frameborder=0></iframe>`,
	}
}

func TestFromJSON(t *testing.T) { testDecode(t, "testdata/out.json", FromJSON) }
func TestFromXML(t *testing.T)  { testDecode(t, "testdata/out.xml", FromXML) }

func testDecode(t *testing.T, name string, fn func(io.Reader) (*Metadata, error)) {
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := fn(f)
	if err != nil {
		t.Fatal(err)
	}
	if ref := reference(); *ref != *m {
		t.Fatalf("got %+v, want %+v", m, ref)
	}
}

func TestProviders(t *testing.T) {
	f, err := os.Open("testdata/providers.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fn, err := Providers(f)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		found bool
		url   string
		dst   string
	}{
		{true, "http://flickr.com/photos/bees/2362225867/",
			"http://www.flickr.com/services/oembed/?url=http%3A%2F%2Fflickr.com%2Fphotos%2Fbees%2F2362225867%2F"},
	}
	for _, test := range tests {
		if got, found := fn(test.url); found != test.found || got != test.dst {
			t.Errorf("url %q\nwant %q\n got %q", test.url, test.dst, got)
		}
	}
}

func TestDiscover(t *testing.T) {
	body := `<!doctype html><head><link rel="alternate" type="application/json+oembed"
	  href="http://flickr.com/services/oembed?url=http%3A%2F%2Fflickr.com%2Fphotos%2Fbees%2F2362225867%2F&format=json"
	    title="Bacon Lollys oEmbed Profile" />`
	url, _, err := Discover(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	want := "http://flickr.com/services/oembed?url=http%3A%2F%2Fflickr.com%2Fphotos%2Fbees%2F2362225867%2F&format=json"
	if url != want {
		t.Fatalf("got %q, want %q", url, want)
	}
}
