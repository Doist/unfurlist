package unfurlist

import (
	"testing"
)

func TestTitleParser(t *testing.T) {
	for i, c := range titleTestCases {
		title, err := findTitle([]byte(c.body))
		if err != nil {
			t.Errorf("case %d failed: %v", i, err)
			continue
		}
		if title != c.want {
			t.Errorf("case %d mismatch: %q != %q", i, title, c.want)
		}
	}
}

var titleTestCases = []struct {
	body string
	want string
}{
	{"<html><title>Hello</title></html>", "Hello"},
	{"<html><TITLE>Hello</TITLE></html>", "Hello"},
	{"<html><title>Hello\n</title></html>", "Hello\n"},
}
