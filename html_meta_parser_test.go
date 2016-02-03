package unfurlist

import (
	"io/ioutil"
	"testing"
)

func TestTitleParser__multibyte1(t *testing.T) {
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

func TestTitleParser__multibyte2(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/japanese")
	if err != nil {
		t.Fatal(err)
	}
	title, err := findTitle(data)
	if err != nil {
		t.Fatal(err)
	}
	want := `【楽天市場】テレビ台【ALTER／アルター】コーナータイプ【ＴＶ台】薄型ＴＶ３７型対応 ＡＶ収納【ＡＶボード】【コーナーボード】【幅１００】◆代引不可★一部組立【駅伝_中_四】：インテリア雑貨通販 H-collection`
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

func BenchmarkTitleParser(b *testing.B) {
	for j := 0; j < b.N; j++ {
		for i, c := range titleTestCases {
			title, err := findTitle([]byte(c.body))
			if err != nil {
				b.Fatalf("case %d failed: %v", i, err)
			}
			if title != c.want {
				b.Fatalf("case %d mismatch: %q != %q", i, title, c.want)
			}
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
