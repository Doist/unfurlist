package unfurlist

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
)

// imageDimensions tries to retrieve enough of image to get its dimensions. If
// provided client is nil, http.DefaultClient is used.
func imageDimensions(imageUrl string, client *http.Client) (width, height int, err error) {
	switch {
	case strings.HasPrefix(imageUrl, "http"):
	case strings.HasPrefix(imageUrl, "//"):
		// most probably scheme-independent url, use http as fallback
		imageUrl = "http:" + imageUrl
	default:
		return 0, 0, errors.New("unsupported image url")
	}
	cl := client
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Get(imageUrl)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return 0, 0, errors.New(resp.Status)
	}
	switch ct := strings.ToLower(resp.Header.Get("Content-Type")); ct {
	case "image/jpeg", "image/png", "image/gif":
	default:
		// for broken servers responding with image/png;charset=UTF-8
		// (i.e. www.evernote.com)
		if strings.HasPrefix(ct, "image/jpeg") ||
			strings.HasPrefix(ct, "image/png") ||
			strings.HasPrefix(ct, "image/gif") {
			break
		}
		return 0, 0, fmt.Errorf("unsupported content-type %q", ct)
	}
	cfg, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}
