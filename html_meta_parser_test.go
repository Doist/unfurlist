package unfurlist

import (
	"testing"
)

func TestTitleParser(t *testing.T) {
	title, _ := findTitle("<html>\n<title>Hello</title></html>")
	want := "Hello"

	if title != want {
		t.Errorf("Title not found: %d != %d", title, want)
	} 
}
