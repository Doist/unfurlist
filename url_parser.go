package unfurlist

import (
	"regexp"
	"strings"
)

// reUrls matches sequence of characters described by RFC 3986 having http:// or
// https:// prefix
var reUrls = regexp.MustCompile(`https?://[%:/?#\[\]@!$&'\(\)*+,;=\pL\pN._~-]+`)

// ParseURLs tries to extract url-like (http/https scheme only) substrings from
// given text. Results may not be proper urls, since only sequence of matched
// characters are searched for. This function is optimized for extraction of
// urls from plain text where it can be mixed with punctuation symbols: trailing
// symbols []()<>,;. are removed, but // trailing >]) are left if any opening
// <[( is found inside url.
func ParseURLs(content string) []string {
	const punct = `[]()<>,;.`
	res := reUrls.FindAllString(content, -1)
	for i, s := range res {
		// remove all combinations of trailing >)],. characters only if
		// no similar characters were found somewhere in the middle
		if idx := strings.IndexAny(s, punct); idx < 0 {
			continue
		}
	cleanLoop:
		for {
			idx2 := strings.LastIndexAny(s, punct)
			if idx2 != len(s)-1 {
				break
			}
			switch s[idx2] {
			case ')':
				if strings.Index(s, `(`) > 0 {
					break cleanLoop
				}
			case ']':
				if strings.Index(s, `[`) > 0 {
					break cleanLoop
				}
			case '>':
				if strings.Index(s, `<`) > 0 {
					break cleanLoop
				}
			}
			s = s[:idx2]
		}
		res[i] = s
	}
	return res
}
