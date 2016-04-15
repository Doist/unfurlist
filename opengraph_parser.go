// Implements the basic Open Graph parser ( http://ogp.me/ )
// Currently we only parse Title, Description, Type and the first Image

package unfurlist

import (
	"bytes"
	"net/http"
	"strings"

	"golang.org/x/net/html/charset"

	"github.com/dyatlov/go-opengraph/opengraph"
)

func openGraphParseHTML(h *unfurlHandler, result *unfurlResult, htmlBody []byte, ct string) bool {
	if !strings.HasPrefix(http.DetectContentType(htmlBody), "text/html") {
		return false
	}
	// use explicit content type received from headers here but not the one returned by
	// http.DetectContentType because this function scans only first 512
	// bytes and can report content as "text/html; charset=utf-8" even for
	// bodies having characters outside utf8 range later; use
	// charset.NewReader that relies on charset.DetermineEncoding which
	// implements more elaborate encoding detection specific to html content
	bodyReader, err := charset.NewReader(bytes.NewReader(htmlBody), ct)
	if err != nil {
		return false
	}
	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(bodyReader)
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
