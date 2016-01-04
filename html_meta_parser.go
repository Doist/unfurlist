// Implements a basic HTML parser that just checks <title>
// It also annotates mime Type if possible

package unfurlist

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/htmlindex"
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
	if utf8.Valid(htmlBody) {
		node, err := html.Parse(bytes.NewReader(htmlBody))
		if err != nil {
			return "", err
		}
		if t, ok := getFirstElement(node, "title"); ok {
			return t, nil
		}
		return title, errNoTitleTag
	}
	// body is not valid utf8, but was (expectedly) detected as "text/*"
	// mime-type by caller of this function, so it most probably one of the
	// multibyte encoding html. Try to parse this as html and extract `meta
	// charset` attribute to convert data to utf8.
	node, err := html.Parse(bytes.NewReader(htmlBody))
	if err != nil {
		return "", err
	}
	charset := htmlCharset(node)
	if charset == "" {
		return "", errors.New("cannot detect multibyte document charset")
	}
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return "", err
	}
	// re-parse document as utf8
	node, err = html.Parse(enc.NewDecoder().Reader(bytes.NewReader(htmlBody)))
	if err != nil {
		return "", err
	}
	if t, ok := getFirstElement(node, "title"); ok {
		return t, nil
	}
	return title, errNoTitleTag
}

// htmlCharset tries to find first 'meta charset=xxx' attribute and extract
// charset as a string
func htmlCharset(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "meta" {
		for _, a := range n.Attr {
			if a.Key == "charset" {
				return a.Val
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if s := htmlCharset(c); s != "" {
			return s
		}
	}
	return ""
}

// getFirstElement returns flattened content of first found element of given
// type
func getFirstElement(n *html.Node, element string) (t string, found bool) {
	if n.Type == html.ElementNode && n.Data == element {
		return flatten(n), true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		t, found = getFirstElement(c, element)
		if found {
			return
		}
	}
	return
}

// flatten returns flattened text content of html node
func flatten(n *html.Node) (res string) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res += flatten(c)
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	return
}

var (
	errNoTitleTag = errors.New("no title tag found")
)
