// Implements the basic Open Graph parser ( http://ogp.me/ )
// Currently we only parse Title, Description, Type and the first Image

package unfurlist

import (
	"bytes"
	"strings"

	"github.com/dyatlov/go-opengraph/opengraph"
)

func OpenGraphParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody []byte) bool {
	og := opengraph.NewOpenGraph()
	err := og.ProcessHTML(bytes.NewReader(htmlBody))
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
