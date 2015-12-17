package unfurlist

import (
	"regexp"
)

var ReUrls = regexp.MustCompile(`https?://[^\s]+`)

func ParseURLs(content string) []string {
	return ReUrls.FindAllString(content, -1)
}
