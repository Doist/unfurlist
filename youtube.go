package unfurlist

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artyom/oembed"
)

// youtubeFetcher that retrieves metadata directly from
// https://www.youtube.com/oembed endpoint.
//
// This is only needed because sometimes youtube may return captcha-walled
// response that does not include oembed endpoint address as part of such html
// page.
func youtubeFetcher(u *url.URL) (*Metadata, bool) {
	if !(u.Host == "www.youtube.com" && u.Path == "/watch" && strings.HasPrefix(u.RawQuery, "v=")) {
		return nil, false
	}
	// TODO: refactor to derive ctx from request (requires FetchFunc signature
	// change)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	const endpointPrefix = `https://www.youtube.com/oembed?format=json&url=`
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointPrefix+url.QueryEscape(u.String()), nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "unfurlist (https://github.com/Doist/unfurlist)")
	resp, err := http.DefaultClient.Do(req)
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
