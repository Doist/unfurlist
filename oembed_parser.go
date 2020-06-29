package unfurlist

import (
	"context"
	"net/http"

	"github.com/artyom/oembed"
)

func fetchOembed(ctx context.Context, url string, fn func(context.Context, string) (*http.Response, error)) (*unfurlResult, error) {
	resp, err := fn(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	meta, err := oembed.FromResponse(resp)
	if err != nil {
		return nil, err
	}
	res := &unfurlResult{
		Title:    meta.Title,
		SiteName: meta.Provider,
		Type:     string(meta.Type),
		HTML:     meta.HTML,
		Image:    meta.Thumbnail,
	}
	if meta.Type == oembed.TypePhoto && meta.URL != "" {
		res.Image = meta.URL
	}
	return res, nil
}
