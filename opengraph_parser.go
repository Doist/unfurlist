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

func openGraphParseHTML(chunk *pageChunk) *unfurlResult {
	if !strings.HasPrefix(http.DetectContentType(chunk.data), "text/html") {
		return nil
	}
	// use explicit content type received from headers here but not the one returned by
	// http.DetectContentType because this function scans only first 512
	// bytes and can report content as "text/html; charset=utf-8" even for
	// bodies having characters outside utf8 range later; use
	// charset.NewReader that relies on charset.DetermineEncoding which
	// implements more elaborate encoding detection specific to html content
	bodyReader, err := charset.NewReader(bytes.NewReader(chunk.data), chunk.ct)
	if err != nil {
		return nil
	}
	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(bodyReader)
	if err != nil {
		return nil
	}
	res := &unfurlResult{
		Type:        og.Type,
		Title:       og.Title,
		Description: og.Description,
		SiteName:    og.SiteName,
	}
	if len(og.Images) > 0 {
		res.Image = og.Images[0].URL
	}
	if isXStatusPage(chunk) {
		res.SiteName = "X"
		if isXProfileImageURL(res.Image) {
			res.Image = ""
		}
	}
	return res
}

func isXStatusPage(chunk *pageChunk) bool {
	if chunk == nil || chunk.url == nil {
		return false
	}

	host := strings.TrimPrefix(strings.ToLower(chunk.url.Hostname()), "www.")
	return (host == "x.com" || host == "twitter.com" || host == "mobile.twitter.com") &&
		strings.Contains(chunk.url.Path, "/status/")
}

func isXProfileImageURL(image string) bool {
	return strings.HasPrefix(image, "https://pbs.twimg.com/profile_images/") ||
		strings.HasPrefix(image, "http://pbs.twimg.com/profile_images/")
}
