package unfurlist

import (
	"fmt"
	"testing"
)

func ExampleParseURLs() {
	text := `This text contains various urls mixed with different reserved per rfc3986 characters:
	http://google.com, https://doist.com/#about (also see https://todoist.com), <http://example.com/foo>,
	[markdown](http://daringfireball.net/projects/markdown/),
	http://marvel-movies.wikia.com/wiki/The_Avengers_(film), https://pt.wikipedia.org/wiki/Mamão.
	http://example.com/?param=foo;bar
	`
	for _, u := range ParseURLs(text) {
		fmt.Println(u)
	}
	// Output:
	// http://google.com
	// https://doist.com/#about
	// https://todoist.com
	// http://example.com/foo
	// http://daringfireball.net/projects/markdown/
	// http://marvel-movies.wikia.com/wiki/The_Avengers_(film)
	// https://pt.wikipedia.org/wiki/Mamão
	// http://example.com/?param=foo;bar
}

func TestBasicURLs(t *testing.T) {
	got := ParseURLs("Testing this out http://doist.com/#about https://todoist.com/chrome")
	want := []string{"http://doist.com/#about", "https://todoist.com/chrome"}

	if len(got) != len(want) {
		t.Errorf("Length not the same got: %d != want: %d", len(got), len(want))
	} else {
		for i := 0; i < len(want); i++ {
			if got[i] != want[i] {
				t.Errorf("%q != %s", got, want)
			}
		}
	}
}

func TestBugURL(t *testing.T) {
	got := ParseURLs("Testing this out Bug report http://f.cl.ly/items/000V0N1B31283s3O350q/Screen%20Shot%202015-12-22%20at%2014.49.28.png")
	want := []string{"http://f.cl.ly/items/000V0N1B31283s3O350q/Screen%20Shot%202015-12-22%20at%2014.49.28.png"}

	if len(got) != len(want) {
		t.Errorf("Length not the same got: %d != want: %d", len(got), len(want))
	} else {
		for i := 0; i < len(want); i++ {
			if got[i] != want[i] {
				t.Errorf("%q != %s", got, want)
			}
		}
	}
}
