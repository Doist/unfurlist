// Implements the oEmbed parser ( http://oembed.com/ )
// Currently we only parse Title, Description, Type and ThumbnailURL

package unfurlist

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dyatlov/go-oembed/oembed"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

func oembedParseURL(h *unfurlHandler, u string) *unfurlResult {
	item := h.OembedParser.FindItem(u)
	if item == nil {
		return nil
	}
	info, err := item.FetchOembed(u, h.HTTPClient)
	if err != nil || info.Status >= 300 {
		return nil
	}
	return &unfurlResult{
		Title:       info.Title,
		Type:        info.Type,
		Description: info.Description,
		Image:       info.ThumbnailURL,
	}
}

func oembedDiscoverable(ctx context.Context, client *http.Client, chunk *pageChunk) (*unfurlResult, error) {
	if chunk == nil || len(chunk.data) == 0 || !strings.HasPrefix(chunk.ct, "text/html") {
		return nil, nil
	}
	link, err := extractOembedLink(chunk.data, chunk.ct)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response code: %q", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return nil, fmt.Errorf("unsupported Content-Type: %q", ct)
	}
	info := oembed.NewInfo()
	if err := info.FillFromJSON(resp.Body); err != nil {
		return nil, err
	}
	if info.Title == "" && info.Type == "" &&
		info.Description == "" && info.ThumbnailURL == "" {
		return nil, nil
	}
	return &unfurlResult{
		Title:       info.Title,
		Type:        info.Type,
		Description: info.Description,
		Image:       info.ThumbnailURL,
	}, nil
}

func extractOembedLink(htmlBody []byte, ct string) (string, error) {
	bodyReader, err := charset.NewReader(bytes.NewReader(htmlBody), ct)
	if err != nil {
		return "", err
	}
	z := html.NewTokenizer(bodyReader)
tokenize:
	for {
		switch tt := z.Next(); tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return "", errNoMetadataFound
			}
			return "", z.Err()
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				return "", errNoMetadataFound
			case atom.Link:
				var link string
				var isOembedLink bool
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "type":
						if !bytes.Equal(v, []byte("application/json+oembed")) {
							continue tokenize
						}
						isOembedLink = true
					case "href":
						link = string(v)
					}
				}
				if isOembedLink && link != "" {
					return link, nil
				}
			}
		}
	}
	return "", errNoMetadataFound
}
