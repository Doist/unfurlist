// Implements a basic HTML parser that just checks <title>
// It also annotates mime Type if possible

package unfurlist

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

func basicParseHTML(chunk *pageChunk) *unfurlResult {
	result := new(unfurlResult)
	result.Type = http.DetectContentType(chunk.data)
	switch {
	case strings.HasPrefix(result.Type, "image/"):
		result.Type = "image"
		result.Image = chunk.url.String()
	case strings.HasPrefix(result.Type, "text/"):
		result.Type = "website"
		// pass Content-Type from response headers as it may have
		// charset definition like "text/html; charset=windows-1251"
		if title, desc, err := extractData(chunk.data, chunk.ct); err == nil {
			result.Title = title
			result.Description = desc
		}
	case strings.HasPrefix(result.Type, "video/"):
		result.Type = "video"
	}
	return result
}

func extractData(htmlBody []byte, ct string) (title, description string, err error) {
	bodyReader, err := charset.NewReader(bytes.NewReader(htmlBody), ct)
	if err != nil {
		return "", "", err
	}
	z := html.NewTokenizer(bodyReader)
tokenize:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				goto finish
			}
			return "", "", z.Err()
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				goto finish // title/meta should preceed body tag
			case atom.Title:
				if title != "" {
					continue
				}
				if tt := z.Next(); tt == html.TextToken {
					title = string(z.Text())
					if description != "" {
						goto finish
					}
				}
			case atom.Meta:
				if description != "" {
					continue
				}
				var content []byte
				var isDescription bool
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "name":
						if !bytes.Equal(v, []byte("description")) {
							continue tokenize
						}
						isDescription = true
					case "content":
						content = v
					}
				}
				if isDescription && len(content) > 0 {
					description = string(content)
					if title != "" {
						goto finish
					}
				}
			}
		}
	}
finish:
	if title != "" || description != "" {
		return title, description, nil
	}
	return "", "", errNoMetadataFound
}

var (
	errNoMetadataFound = errors.New("no metadata found")
)
