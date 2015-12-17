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

func BasicParseParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody []byte) serviceResult {
	serviceResult := serviceResult{Result: result, HasMatch: false}

	title, err := findTitle(htmlBody)
	if err == nil {
		result.Title = title
		serviceResult.HasMatch = true
	}

	result.Type = http.DetectContentType(htmlBody)

	if strings.Index(result.Type, "image/") != -1 {
		result.Type = "image"
		result.Image = result.URL
	} else if strings.Index(result.Type, "text/") != -1 {
		result.Type = "website"
	} else if strings.Index(result.Type, "video/") != -1 {
		result.Type = "video"
	}

	return serviceResult
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
