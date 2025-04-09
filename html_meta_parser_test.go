package unfurlist

import (
	"os"
	"testing"
)

func TestExtractData_explicitCharset(t *testing.T) {
	// this file has its charset defined at around ~1600 bytes, WHATWG
	// charset detection algorithm [1] fails here as it only scans first
	// 1024 bytes, so we also need to rely on server-provided charset
	// parameter from Content-Type header
	//
	// [1]: https://html.spec.whatwg.org/multipage/syntax.html#determining-the-character-encoding
	data, err := os.ReadFile("testdata/no-charset-in-first-1024bytes")
	if err != nil {
		t.Fatal(err)
	}
	title, _, err := extractData(data, "text/html; charset=windows-1251")
	if err != nil {
		t.Fatal(err)
	}
	want := `Кубань и Адыгея объединят усилия по созданию курорта "Лагонаки"`
	if title != want {
		t.Fatalf("unexpected title: got %q, want %q", title, want)
	}
}

func TestExtractData_multibyte1(t *testing.T) {
	data, err := os.ReadFile("testdata/korean")
	if err != nil {
		t.Fatal(err)
	}
	title, _, err := extractData(data, "text/html")
	if err != nil {
		t.Fatal(err)
	}
	want := `심장정지 환자 못살리는 119 구급차 - 1등 인터넷뉴스 조선닷컴 - 의료ㆍ보건`
	if title != want {
		t.Fatalf("unexpected title: got %q, want %q", title, want)
	}
}

func TestExtractData_multibyte2(t *testing.T) {
	data, err := os.ReadFile("testdata/japanese")
	if err != nil {
		t.Fatal(err)
	}
	title, _, err := extractData(data, "text/html")
	if err != nil {
		t.Fatal(err)
	}
	want := `【楽天市場】テレビ台【ALTER／アルター】コーナータイプ【ＴＶ台】薄型ＴＶ３７型対応 ＡＶ収納【ＡＶボード】【コーナーボード】【幅１００】◆代引不可★一部組立【駅伝_中_四】：インテリア雑貨通販 H-collection`
	if title != want {
		t.Fatalf("unexpected title: got %q, want %q", title, want)
	}
}

func TestExtractData(t *testing.T) {
	for i, c := range titleTestCases {
		title, _, err := extractData([]byte(c.body), "text/html")
		if err != nil {
			t.Errorf("case %d failed: %v", i, err)
			continue
		}
		if title != c.want {
			t.Errorf("case %d mismatch: %q != %q", i, title, c.want)
		}
	}
}

func TestExtractData_full(t *testing.T) {
	body := `<html>
	<meta name="keywords" content="test">
	<meta name="description" content="hello page">
	<meta name="description" content="ignored">
	<title>Hello</title>
	</html>
	`
	title, desc, err := extractData([]byte(body), "text/html")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Hello"; title != want {
		t.Errorf("got title %q, want %q", title, want)
	}
	if want := "hello page"; desc != want {
		t.Errorf("got description %q, want %q", desc, want)
	}
}

func BenchmarkExtractData(b *testing.B) {
	for b.Loop() {
		for i, c := range titleTestCases {
			title, _, err := extractData([]byte(c.body), "text/html")
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
