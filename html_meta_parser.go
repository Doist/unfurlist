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

func BasicParseParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody []byte) bool {
	result.Type = http.DetectContentType(htmlBody)
	switch {
	case strings.HasPrefix(result.Type, "image/"):
		result.Type = "image"
		result.Image = result.URL
	case strings.HasPrefix(result.Type, "text/"):
		result.Type = "website"
		if title, err := findTitle(htmlBody); err == nil {
			result.Title = title
		}
	case strings.HasPrefix(result.Type, "video/"):
		result.Type = "video"
	}
	return true
}

func findTitle(htmlBody []byte) (title string, err error) {
	bodyReader, err := charset.NewReader(bytes.NewReader(htmlBody), "text/html")
	if err != nil {
		return "", err
	}
	z := html.NewTokenizer(bodyReader)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				goto notFound
			}
			return "", z.Err()
		case html.StartTagToken:
			name, _ := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				goto notFound // title should preceed body tag
			case atom.Title:
				if tt := z.Next(); tt == html.TextToken {
					return string(z.Text()), nil
				} else {
					goto notFound
				}
			}
		}
	}
notFound:
	return "", errNoTitleTag
}

var (
	errNoTitleTag = errors.New("no title tag found")
)
