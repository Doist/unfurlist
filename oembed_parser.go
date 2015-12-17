// Implements the oEmbed parser ( http://oembed.com/ )
// Currently we only parse Title, Description, Type and ThumbnailURL

package unfurlist

func OembedParseUrl(h *unfurlHandler, result *unfurlResult) serviceResult {
	serviceResult := serviceResult{Result: result, HasMatch: false}
	item := h.Config.OembedParser.FindItem(result.URL)

	if item != nil {
		info, err := item.FetchOembed(result.URL, nil)
		if err == nil {
			if info.Status < 300 {
				serviceResult.HasMatch = true

				result.Title = info.Title
				result.Type = info.Type
				result.Description = info.Description
				result.Image = info.ThumbnailURL
			}
		}
	}

	return serviceResult
}
