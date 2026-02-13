package unfurlist

import (
	"fmt"
	"testing"
)

func ExampleParseURLs() {
	text := `This text contains various urls mixed with different reserved per rfc3986 characters:
	http://google.com, https://doist.com/#about (also see https://todoist.com), <http://example.com/foo>,
	**[markdown](http://daringfireball.net/projects/markdown/)**,
	http://marvel-movies.wikia.com/wiki/The_Avengers_(film), https://pt.wikipedia.org/wiki/Mamão.
	https://docs.live.net/foo/?section-id={D7CEDACE-AEFB-4B61-9C63-BDE05EEBD80A},
	http://example.com/?param=foo;bar
	HTTPS://EXAMPLE.COM/UPPERCASE
	hTtP://example.com/mixedCase
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
	// https://docs.live.net/foo/?section-id={D7CEDACE-AEFB-4B61-9C63-BDE05EEBD80A}
	// http://example.com/?param=foo;bar
	// HTTPS://EXAMPLE.COM/UPPERCASE
	// hTtP://example.com/mixedCase
}

func TestParseURLs__unique(t *testing.T) {
	got := ParseURLs("Only two unique urls should be extracted from this text: http://google.com, http://twitter.com, http://google.com")
	want := []string{"http://google.com", "http://twitter.com"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i, v := range got {
		if v != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

func TestBasicURLs(t *testing.T) {
	got := ParseURLs("Testing this out http://doist.com/#about https://todoist.com/chrome")
	want := []string{"http://doist.com/#about", "https://todoist.com/chrome"}

	if len(got) != len(want) {
		t.Errorf("Length not the same got: %d != want: %d", len(got), len(want))
	} else {
		for i := range want {
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
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%q != %s", got, want)
			}
		}
	}
}

func TestValidURL(t *testing.T) {
	testCases := []struct {
		u   string
		res bool
	}{
		{"https://example.com/path?multi+word+escaped+query", true},
		{"https://example.com/path?unescaped query", false},
		{"ftp://example.com/unsupported/scheme", false},
		{"", false},
		{"https://example.com/path", true},
		{"https:///path", false},
	}
	for _, tc := range testCases {
		if validURL(tc.u) != tc.res {
			t.Fatalf("validURL(%q)==%t, want %t", tc.u, !tc.res, tc.res)
		}
	}
}

func TestParseMarkdownURLs(t *testing.T) {
	text := `Implicit url: http://example.com/1, [explicit url](http://example.com/2).

This url should be skipped ` + "`http://example.com/3`" + `, as well as the one inside code block:

	preformatted text block with url: http://example.com/4

Another paragraph with implicit link http://example.com/5.
	`
	got := parseMarkdownURLs(text, 10)
	want := []string{"http://example.com/1", "http://example.com/2", "http://example.com/5"}
	if len(got) != len(want) {
		t.Fatalf("want: %v, got: %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%d: want %q, got %q", i, want[i], got[i])
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips utm params",
			input: "https://example.com/page?utm_source=twitter&utm_medium=social",
			want:  "https://example.com/page",
		},
		{
			name:  "preserves non-tracking params",
			input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			want:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			name:  "mixed tracking and real params",
			input: "https://example.com/page?utm_source=twitter&id=42",
			want:  "https://example.com/page?id=42",
		},
		{
			name:  "no query string unchanged",
			input: "https://www.imdb.com/title/tt1234567/",
			want:  "https://www.imdb.com/title/tt1234567/",
		},
		{
			name:  "all tracking params removed",
			input: "https://example.com/page?utm_source=x&fbclid=y&gclid=z",
			want:  "https://example.com/page",
		},
		{
			name:  "strips imdb tracking params",
			input: "https://www.imdb.com/title/tt1234567/?ref_=nv_sr_srsg_0&pf_rd_m=abc",
			want:  "https://www.imdb.com/title/tt1234567/",
		},
		{
			name:  "strips facebook click id preserves others",
			input: "https://example.com/article?fbclid=abc123&page=2",
			want:  "https://example.com/article?page=2",
		},
		{
			name:  "non-http scheme unchanged",
			input: "ftp://example.com/file?utm_source=x",
			want:  "ftp://example.com/file?utm_source=x",
		},
		{
			name:  "invalid url unchanged",
			input: "not a url at all",
			want:  "not a url at all",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeURL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

var escape []string

func BenchmarkMarkdownURLs(b *testing.B) {
	text := `Implicit url: http://example.com/1, [explicit url](http://example.com/2).

This url should be skipped ` + "`http://example.com/3`" + `, as well as the one inside code block:

	preformatted text block with url: http://example.com/4

Another paragraph with implicit link http://example.com/5.
	`
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		escape = parseMarkdownURLs(text, 10)
	}
}
