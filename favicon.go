package unfurlist

import (
	"bytes"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

// extractFaviconLink parses html data in search of the first <link rel="icon"
// ...> element and returns value of its href attribute.
func extractFaviconLink(htmlBody []byte, ct string) string {
	bodyReader, err := charset.NewReader(bytes.NewReader(htmlBody), ct)
	if err != nil {
		return ""
	}
	z := html.NewTokenizer(bodyReader)
tokenize:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return ""
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				return ""
			case atom.Link:
				var href string
				var isIconLink bool
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "rel":
						if !bytes.EqualFold(v, []byte("icon")) {
							continue tokenize
						}
						isIconLink = true
					case "href":
						href = string(v)
					}
				}
				if isIconLink && href != "" {
					return href
				}
			}
		}
	}
}
