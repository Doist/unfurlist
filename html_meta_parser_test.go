package unfurlist

import (
	"io/ioutil"
	"testing"
)

func TestTitleParser_multibyte(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/korean")
	if err != nil {
		t.Fatal(err)
	}
	title, err := findTitle(data)
	if err != nil {
		t.Fatal(err)
	}
	want := `심장정지 환자 못살리는 119 구급차 - 1등 인터넷뉴스 조선닷컴 - 의료ㆍ보건`
	if title != want {
		t.Fatalf("unexpected title: got %q, want %q", title, want)
	}
}

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
