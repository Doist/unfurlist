package unfurlist

import (
	"regexp"
	"strings"
)

// reUrls matches sequence of characters described by RFC 3986 having http:// or
// https:// prefix. It actually allows superset of characters from RFC 3986,
// allowing some most commonly used characters like {}, etc.
var reUrls = regexp.MustCompile(`https?://[%:/?#\[\]@!$&'\(\){}*+,;=\pL\pN._~-]+`)

// ParseURLs tries to extract unique url-like (http/https scheme only) substrings from
// given text. Results may not be proper urls, since only sequence of matched
// characters are searched for. This function is optimized for extraction of
// urls from plain text where it can be mixed with punctuation symbols: trailing
// symbols []()<>,;. are removed, but // trailing >]) are left if any opening
// <[( is found inside url.
func ParseURLs(content string) []string { return parseURLsMax(content, -1) }

func parseURLsMax(content string, maxItems int) []string {
	const punct = `[]()<>{},;.*_`
	res := reUrls.FindAllString(content, maxItems)
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
			case '}':
				if strings.Index(s, `{`) > 0 {
					break cleanLoop
				}
			}
			s = s[:idx2]
		}
		res[i] = s
	}
	out := res[:0]
	seen := make(map[string]struct{})
	for _, v := range res {
		if _, ok := seen[v]; ok {
			continue
		}
		out = append(out, v)
		seen[v] = struct{}{}
	}
	return out
}
