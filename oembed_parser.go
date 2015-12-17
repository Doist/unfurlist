// Implements the oEmbed parser ( http://oembed.com/ )
// Currently we only parse Title, Description, Type and ThumbnailURL

package unfurlist

func OembedParseUrl(h *unfurlHandler, result *unfurlResult) bool {
	item := h.Config.OembedParser.FindItem(result.URL)
	if item == nil {
		return false
	}
	info, err := item.FetchOembed(result.URL, nil)
	if err != nil || info.Status >= 300 {
		return false
	}
	result.Title = info.Title
	result.Type = info.Type
	result.Description = info.Description
	result.Image = info.ThumbnailURL

	return true
}
