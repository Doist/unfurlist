package unfurlist

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register supported image types
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"strings"
)

var errEmptyImageURL = errors.New("empty image url")

// absoluteImageUrl makes imageUrl absolute if it's not. Image url can either be
// relative or schemaless url.
func absoluteImageURL(originURL, imageURL string) (string, error) {
	if imageURL == "" {
		return "", errEmptyImageURL
	}
	if strings.HasPrefix(imageURL, "http") {
		return imageURL, nil
	}
	iu, err := url.Parse(imageURL)
	if err != nil {
		return "", err
	}
	switch iu.Scheme {
	case "http", "https", "":
	default:
		return "", fmt.Errorf("unsupported url scheme %q", iu.Scheme)
	}
	base, err := url.Parse(originURL)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(iu).String(), nil
}

// imageDimensions tries to retrieve enough of image to get its dimensions. If
// provided client is nil, http.DefaultClient is used.
func imageDimensions(imageURL string, client *http.Client) (width, height int, err error) {
	cl := client
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Get(imageURL)
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
