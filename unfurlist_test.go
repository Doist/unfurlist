package unfurlist

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenGraph(t *testing.T) {
	result := doRequest("/?content=Test+http://techcrunch.com/2015/11/09/basic-income-createathon/", t)
	if len(result) != 1 {
		t.Fatalf("invalid result length: %v", result)
	}

	want := "Robots To Eat All The Jobs? Hackers, Policy Wonks Collaborate On A Basic Income Createathon This Weekend – TechCrunch"
	if result[0].Title != want {
		t.Errorf("unexpected Title, want %q, got %q", want, result[0].Title)
	}

	want = "https://techcrunch.com/wp-content/uploads/2015/11/basic-income-createathon.jpg?w=602"
	if result[0].Image != want {
		t.Errorf("unexpected Image, want %q, got %q", want, result[0].Image)
	}
}

func TestOpenGraphTwitter(t *testing.T) {
	result := doRequest("/?content=Test+https://twitter.com/amix3k/status/1399300280206909440", t)
	if len(result) != 1 {
		t.Fatalf("invalid result length: %v", result)
	}

	want := "My current meeting schedule this week"
	if !strings.Contains(result[0].Description, want) {
		t.Errorf("unexpected Description, want %q, got %q", want, result[0].Description)
	}
}

func TestHtml(t *testing.T) {
	result := doRequest("/?content=https://news.ycombinator.com/", t)
	if len(result) != 1 {
		t.Fatalf("invalid result length: %v", result)
	}

	want := "Hacker News"
	if result[0].Title != want {
		t.Errorf("unexpected Title, want %q, got %q", want, result[0].Title)
	}

	want = ""
	if result[0].Image != want {
		t.Errorf("unexpected Image, want %q, got %q", want, result[0].Image)
	}

	want = "website"
	if result[0].Type != want {
		t.Errorf("unexpected Type, want %q, got %q", want, result[0].Type)
	}
}

func TestUnfurlist__multibyteHTML(t *testing.T) {
	res := doRequest("/?content=http://news.chosun.com/site/data/html_dir/2009/09/24/2009092401755.html", t)
	want := `심장정지 환자 못살리는 119 구급차`
	if len(res) != 1 {
		t.Fatalf("invalid result length: %v", res)
	}
	if res[0].Title != want {
		t.Errorf("unexpected Title, want %q, got %q", want, res[0].Title)
	}
}

func doRequest(url string, t *testing.T) []unfurlResult {
	pp := newPipePool()
	defer pp.Close()
	go http.Serve(pp, http.HandlerFunc(replayHandler))
	handler := New(WithHTTPClient(&http.Client{
		Transport: &http.Transport{
			Dial:    pp.Dial,
			DialTLS: pp.Dial,
		}}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", url, nil)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("invalid status code: %v", w.Code)
		return nil
	}

	var result []unfurlResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Result isn't JSON %v", w.Body.String())
		return nil
	}

	return result
}

func TestUnfurlist__singleInFlightRequest(t *testing.T) {
	pp := newPipePool()
	defer pp.Close()
	go http.Serve(pp, http.HandlerFunc(replayHandlerSerial(t)))
	handler := New(WithHTTPClient(&http.Client{
		Transport: &http.Transport{
			Dial:    pp.Dial,
			DialTLS: pp.Dial,
		},
	}))

	var wg sync.WaitGroup
	barrier := make(chan struct{})
	for range 3 {
		wg.Add(1)
		go func() {
			w := httptest.NewRecorder()
			<-barrier
			req := httptest.NewRequest("GET", "/?content=https://news.ycombinator.com/", nil)
			handler.ServeHTTP(w, req)
			wg.Done()
		}()
	}
	// ensure multiple calls of unfurlistHandler.processURL() would be done
	// as close to each other as possible
	close(barrier)
	wg.Wait()
}

// replayHandlerSerial returns http.Handler responding with pre-recorded data
// while ensuring that it doesn't process two simultaneous requests for the same
// url
func replayHandlerSerial(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
	inFlight := struct {
		mu   sync.Mutex
		reqs map[string]struct{}
	}{
		reqs: make(map[string]struct{}),
	}
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Host + r.URL.RequestURI()
		inFlight.mu.Lock()
		_, ok := inFlight.reqs[key]
		if ok {
			inFlight.mu.Unlock()
			t.Errorf("request for %q is already in flight", key)
			return
		}
		inFlight.reqs[key] = struct{}{}
		inFlight.mu.Unlock()
		defer func() {
			inFlight.mu.Lock()
			delete(inFlight.reqs, key)
			inFlight.mu.Unlock()
		}()

		d, ok := remoteData[r.Host+r.URL.RequestURI()]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		time.Sleep(10 * time.Millisecond) // increasing chances that multiple goroutines will call handler concurrently
		w.Write([]byte(d))
	}
}

// replayHandler is a http.Handler responding with pre-recorded data
func replayHandler(w http.ResponseWriter, r *http.Request) {
	d, ok := remoteData[r.Host+r.URL.RequestURI()]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// avoid type auto-detecting of saved pages
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(d))
}

// pipePool implements net.Listener interface and provides a Dial() func to dial
// to this listener
type pipePool struct {
	m           sync.RWMutex
	closed      bool
	serverConns chan net.Conn
}

func newPipePool() *pipePool { return &pipePool{serverConns: make(chan net.Conn)} }

func (p *pipePool) Accept() (net.Conn, error) {
	c, ok := <-p.serverConns
	if !ok {
		return nil, errors.New("listener is closed")
	}
	return c, nil
}

func (p *pipePool) Close() error {
	p.m.Lock()
	defer p.m.Unlock()
	if !p.closed {
		close(p.serverConns)
		p.closed = true
	}
	return nil
}
func (p *pipePool) Addr() net.Addr { return phonyAddr{} }

func (p *pipePool) Dial(network, addr string) (net.Conn, error) {
	p.m.RLock()
	defer p.m.RUnlock()
	if p.closed {
		return nil, errors.New("listener is closed")
	}
	c1, c2 := net.Pipe()
	p.serverConns <- c1
	return c2, nil
}

type phonyAddr struct{}

func (a phonyAddr) Network() string { return "pipe" }
func (a phonyAddr) String() string  { return "pipe" }

//go:generate go run remote-data-update.go

//go:embed testdata/remote-dump.json
var remoteDataJson []byte

var remoteData map[string]string

func init() {
	var err error
	if err = json.Unmarshal(remoteDataJson, &remoteData); err != nil {
		panic(err)
	}
}
