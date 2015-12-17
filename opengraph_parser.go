// Implements the basic Open Graph parser ( http://ogp.me/ )
// Currently we only parse Title, Description, Type and the first Image

package unfurlist

import (
	"github.com/dyatlov/go-opengraph/opengraph"
	"strings"
)

func OpenGraphParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody string, resp chan<- serviceResult) {
	serviceResult := serviceResult{Result: result, HasMatch: false}

	og := opengraph.NewOpenGraph()
	err := og.ProcessHTML(strings.NewReader(htmlBody))

	if err != nil || og.Title == "" {
		resp <- serviceResult
		return
	}

	result.Title = og.Title
	result.Description = og.Description
	result.Type = og.Type
	if len(og.Images) > 0 {
		result.Image = og.Images[0].URL
	}
	serviceResult.HasMatch = true

	resp <- serviceResult
}
