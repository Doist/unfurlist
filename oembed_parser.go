// Implements the oEmbed parser ( http://oembed.com/ )
// Currently we only parse Title, Description, Type and ThumbnailURL

package unfurlist

func oembedParseURL(h *unfurlHandler, u string) *unfurlResult {
	item := h.OembedParser.FindItem(u)
	if item == nil {
		return nil
	}
	info, err := item.FetchOembed(u, h.HTTPClient)
	if err != nil || info.Status >= 300 {
		return nil
	}
	return &unfurlResult{
		Title:       info.Title,
		Type:        info.Type,
		Description: info.Description,
		Image:       info.ThumbnailURL,
	}
}
