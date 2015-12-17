// Implements a basic HTML parser that just checks <title>
// It also annotates mime Type if possible

package unfurlist

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

func BasicParseParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody []byte) bool {
	if title, err := findTitle(htmlBody); err == nil {
		result.Title = title
	}
	result.Type = http.DetectContentType(htmlBody)
	switch {
	case strings.HasPrefix(result.Type, "image/"):
		result.Type = "image"
		result.Image = result.URL
	case strings.HasPrefix(result.Type, "text/"):
		result.Type = "website"
	case strings.HasPrefix(result.Type, "video/"):
		result.Type = "video"
	}
	return true
}

func findTitle(htmlBody []byte) (title string, err error) {
	node, err := html.Parse(bytes.NewReader(htmlBody))
	if err != nil {
		return "", err
	}
	if t, ok := getFirstElement(node, "title"); ok {
		return t, nil
	}
	return title, errors.New("no title tag found")
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
