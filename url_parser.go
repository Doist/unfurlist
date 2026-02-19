package unfurlist

import (
	"iter"
	"net/url"
	"regexp"
	"strings"

	"rsc.io/markdown"
)

// reUrls matches sequence of characters described by RFC 3986 having http:// or
// https:// prefix. It actually allows superset of characters from RFC 3986,
// allowing some most commonly used characters like {}, etc.
var reUrls = regexp.MustCompile(`(?i:https?)://[%:/?#\[\]@!$&'\(\){}*+,;=\pL\pN._~-]+`)

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

// validURL returns true if s is a valid absolute url with http/https scheme.
// In addition to verification that s is not empty and url.Parse(s) returns nil
// error, validURL also ensures that query part only contains characters allowed
// by RFC 3986 3.4.
//
// This is required because url.Parse doesn't verify query part of the URI.
func validURL(s string) bool {
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return false
	}
	for _, r := range u.RawQuery {
		// https://tools.ietf.org/html/rfc3986#section-3.4 defines:
		//
		//	query       = *( pchar / "/" / "?" )
		//	pchar         = unreserved / pct-encoded / sub-delims / ":" / "@"
		//	unreserved    = ALPHA / DIGIT / "-" / "." / "_" / "~"
		//	pct-encoded   = "%" HEXDIG HEXDIG
		//	sub-delims    = "!" / "$" / "&" / "'" / "(" / ")"
		//			/ "*" / "+" / "," / ";" / "="
		//
		// check for these
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		default:
			switch r {
			case '/', '?',
				':', '@',
				'-', '.', '_', '~',
				'%', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=':
			default:
				return false
			}
		}
	}
	return true
}

var parser = markdown.Parser{AutoLinkText: true}

func parseMarkdownURLs(content string, maxItems int) []string {
	var out []string
	for link := range docLinks(parser.Parse(content)) {
		if !validURL(link.URL) {
			continue
		}
		out = append(out, link.URL)
		if maxItems != 0 && len(out) == maxItems {
			return out
		}
	}
	return out
}

func docLinks(doc *markdown.Document) iter.Seq[*markdown.Link] {
	var walkLinks func(markdown.Inlines, func(*markdown.Link) bool) bool
	walkLinks = func(inlines markdown.Inlines, yield func(*markdown.Link) bool) bool {
		for _, inl := range inlines {
			switch ent := inl.(type) {
			case *markdown.Strong:
				if !walkLinks(ent.Inner, yield) {
					return false
				}
			case *markdown.Emph:
				if !walkLinks(ent.Inner, yield) {
					return false
				}
			case *markdown.Link:
				if !yield(ent) {
					return false
				}
			}
		}
		return true
	}
	var walkBlocks func(markdown.Block, func(*markdown.Link) bool) bool
	walkBlocks = func(block markdown.Block, yield func(*markdown.Link) bool) bool {
		switch bl := block.(type) {
		case *markdown.Item:
			for _, b := range bl.Blocks {
				if !walkBlocks(b, yield) {
					return false
				}
			}
		case *markdown.List:
			for _, b := range bl.Items {
				if !walkBlocks(b, yield) {
					return false
				}
			}
		case *markdown.Paragraph:
			if !walkLinks(bl.Text.Inline, yield) {
				return false
			}
		case *markdown.Quote:
			for _, b := range bl.Blocks {
				if !walkBlocks(b, yield) {
					return false
				}
			}
		case *markdown.Text:
			if !walkLinks(bl.Inline, yield) {
				return false
			}
		}
		return true
	}

	return func(yield func(*markdown.Link) bool) {
		for _, b := range doc.Blocks {
			if !walkBlocks(b, yield) {
				return
			}
		}
	}
}
