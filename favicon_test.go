package unfurlist

import "testing"

func Test_extractFaviconLink(t *testing.T) {
	table := []struct{ input, want string }{
		{`<html><head><title>foo</title></head><body>`, ""},
		{`<html><head><title>foo</title><link rel='icon' href='https://example.com/favicon.ico'></head><body>`,
			"https://example.com/favicon.ico"},
	}
	for i, tt := range table {
		got := extractFaviconLink([]byte(tt.input), "text/html")
		if got != tt.want {
			t.Errorf("case %d failed:\n got: %q,\nwant: %q,\ninput is:\n%s", i, got, tt.want, tt.input)
		}
	}
}
