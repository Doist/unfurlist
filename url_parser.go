package unfurlist

import (
	"regexp"
)

var reUrls = regexp.MustCompile(`https?://[^\s]+`)

func ParseURLs(content string) []string {
	return reUrls.FindAllString(content, -1)
}
