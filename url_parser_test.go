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
