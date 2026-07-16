package unfurlist

import "testing"

func TestCanonicalInstagramOembedURL(t *testing.T) {
	testCases := []struct {
		name string
		link string
		want string
	}{
		{"post", "https://www.instagram.com/p/Da1GcLiMA-K/", "https://www.instagram.com/p/Da1GcLiMA-K/"},
		{"reel", "https://www.instagram.com/reel/Da1BvizS8Me/", "https://www.instagram.com/reel/Da1BvizS8Me/"},
		{"plural reels", "https://www.instagram.com/reels/Da1BvizS8Me/", "https://www.instagram.com/reel/Da1BvizS8Me/"},
		{"username prefix", "https://www.instagram.com/halfblood.university/reel/Da1BvizS8Me/", "https://www.instagram.com/reel/Da1BvizS8Me/"},
		{"deep post link", "https://www.instagram.com/p/Da1GcLiMA-K/comments/", "https://www.instagram.com/p/Da1GcLiMA-K/"},
		{"query", "https://www.instagram.com/p/Da1GcLiMA-K/comments/?img_index=2", "https://www.instagram.com/p/Da1GcLiMA-K/?img_index=2"},
		{"short host", "https://instagr.am/tv/ABC123/", "https://instagr.am/tv/ABC123/"},
		{"malformed URL", "https://instagram.com/p/%zz", "https://instagram.com/p/%zz"},
		{"unsupported scheme", "ftp://instagram.com/p/ABC123/", "ftp://instagram.com/p/ABC123/"},
		{"user info", "https://user@instagram.com/p/ABC123/", "https://user@instagram.com/p/ABC123/"},
		{"port", "https://instagram.com:443/p/ABC123/", "https://instagram.com:443/p/ABC123/"},
		{"lookalike host", "https://instagram.com.example/p/ABC123/", "https://instagram.com.example/p/ABC123/"},
		{"profile", "https://www.instagram.com/instagram/", "https://www.instagram.com/instagram/"},
		{"stories", "https://www.instagram.com/stories/instagram/123/", "https://www.instagram.com/stories/instagram/123/"},
		{"reels audio", "https://www.instagram.com/reels/audio/123/", "https://www.instagram.com/reels/audio/123/"},
		{"login", "https://www.instagram.com/accounts/login/", "https://www.instagram.com/accounts/login/"},
		{"missing shortcode", "https://www.instagram.com/p/", "https://www.instagram.com/p/"},
		{"other site", "https://www.youtube.com/watch?v=123", "https://www.youtube.com/watch?v=123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalInstagramOembedURL(tc.link); got != tc.want {
				t.Fatalf("canonicalInstagramOembedURL(%q) = %q, want %q", tc.link, got, tc.want)
			}
		})
	}
}
