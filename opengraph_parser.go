// Implements the basic Open Graph parser ( http://ogp.me/ )
// Currently we only parse Title, Description, Type and the first Image

package unfurlist

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/htmlindex"

	"github.com/dyatlov/go-opengraph/opengraph"
)

func OpenGraphParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody []byte) bool {
	var bodyReader io.Reader = bytes.NewReader(htmlBody)
	if !utf8.Valid(htmlBody) {
		node, err := html.Parse(bytes.NewReader(htmlBody))
		if err != nil {
			goto ogProcess
		}
		if enc, err := htmlindex.Get(htmlCharset(node)); err == nil {
			bodyReader = enc.NewDecoder().Reader(bodyReader)
		}
	}
ogProcess:
	og := opengraph.NewOpenGraph()
	err := og.ProcessHTML(bodyReader)
	if err != nil || og.Title == "" {
		return false
	}

	result.Title = og.Title
	result.Description = og.Description
	result.Type = og.Type
	if len(og.Images) > 0 {
		result.Image = og.Images[0].URL
	}

	//--- Do special optimizations for popular services
	url := strings.ToLower(result.URL)

	// Twitter
	if strings.Contains(url, "twitter.com") && strings.Contains(url, "/status/") {
		result.Title, result.Description = result.Description, result.Title
	}

	return true
}
