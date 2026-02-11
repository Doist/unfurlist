package unfurlist

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/artyom/oembed"
)

// youtubeFetcher that retrieves metadata directly from
// https://www.youtube.com/oembed endpoint.
//
// This is only needed because sometimes youtube may return captcha-walled
// response that does not include oembed endpoint address as part of such html
// page.
func youtubeFetcher(ctx context.Context, client *http.Client, u *url.URL) (*Metadata, bool) {
	switch {
	case u.Host == "youtu.be" && len(u.Path) > 2:
	case u.Host == "www.youtube.com" && u.Path == "/watch" && u.Query().Has("v"):
	default:
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	const endpointPrefix = `https://www.youtube.com/oembed?format=json&url=`
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointPrefix+url.QueryEscape(u.String()), nil)
	if err != nil {
		return nil, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	meta, err := oembed.FromResponse(resp)
	if err != nil {
		return nil, false
	}
	return &Metadata{
		Title:       meta.Title,
		Type:        string(meta.Type),
		Image:       meta.Thumbnail,
		ImageWidth:  meta.ThumbnailWidth,
		ImageHeight: meta.ThumbnailHeight,
	}, true
}
