package unfurlist

import (
	"testing"
)

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
