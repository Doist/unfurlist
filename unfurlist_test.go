package unfurlist

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenGraph(t *testing.T) {
	result := doRequest("/?content=Test+http://techcrunch.com/2015/11/09/basic-income-createathon/", t)

	want := "Robots To Eat All The Jobs? Hackers, Policy Wonks Collaborate On A Basic Income Createathon This\u00a0Weekend"
	if result[0].Title != want {
		t.Errorf("Title not valid, %q != %q", want, result[0].Title)
	}

	want = "https://tctechcrunch2011.files.wordpress.com/2015/11/basic-income-createathon.jpg?w=764\u0026h=400\u0026crop=1"
	if result[0].Image != want {
		t.Errorf("Image not valid, %q != %q", want, result[0].Title)
	}
}

func TestOembed(t *testing.T) {
	result := doRequest("/?content=Test+https://www.youtube.com/watch?v=Ey8FzGECjFA", t)

	want := "Jony Ive, J.J. Abrams, and Brian Grazer on Inventing Worlds in a Changing One - FULL CONVERSATION"
	if result[0].Title != want {
		t.Errorf("Title not valid, %q != %q", want, result[0].Title)
	}

	want = "https://i.ytimg.com/vi/Ey8FzGECjFA/hqdefault.jpg"
	if result[0].Image != want {
		t.Errorf("Image not valid, %q != %q", want, result[0].Title)
	}

	want = "video"
	if result[0].Type != want {
		t.Errorf("Type not valid, %q != %q", want, result[0].Title)
	}
}

func TestHtml(t *testing.T) {
	result := doRequest("/?content=https://news.ycombinator.com/", t)

	want := "Hacker News"
	if result[0].Title != want {
		t.Errorf("Title not valid, %q != %q", want, result[0].Title)
	}

	want = ""
	if result[0].Image != want {
		t.Errorf("Image not valid, %q != %q", want, result[0].Title)
	}

	want = "website"
	if result[0].Type != want {
		t.Errorf("Type not valid, %q != %q", want, result[0].Type)
	}
}

func doRequest(url string, t *testing.T) []unfurlResult {
	config := UnfurlConfig{}
	handler := New(&config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", url, nil)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Techcrunch Open graph test didn't return %v", http.StatusOK)
	}

	var result []unfurlResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Result isn't JSON %v", w.Body.String())
	}

	return result
}
