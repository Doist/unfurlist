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
	return &unfurlResult{
		Title: meta.Title,
		Type:  string(meta.Type),
		Image: meta.Thumbnail,
	}, nil
}
