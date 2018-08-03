package unfurlist

import "fmt"

func ExampleprefixMap() {
	pm := newPrefixMap([]string{"https://mail.google.com/mail/", "https://trello.com/c/"})

	urls := []string{
		"http://example.com/index.html",
		"https://mail.google.com/mail/u/0/#inbox",
		"https://trello.com/c/a12def34",
	}
	for _, u := range urls {
		fmt.Printf("%q\t%v\n", u, pm.Match(u))
	}
	// Output:
	// "http://example.com/index.html"	false
	// "https://mail.google.com/mail/u/0/#inbox"	true
	// "https://trello.com/c/a12def34"	true
}
