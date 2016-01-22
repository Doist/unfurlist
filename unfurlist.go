// unfurlist is a simple service that unfurls URLs and provides more information about them.
//
// The current version supports Open Graph and oEmbed formats, Twitter card format is also planned.
// If the URL does not support common formats it falls back to looking at HTML tags such as
// <title>.
//
// The endpoint accepts GET requests with `content` as the main argument.
// It then returns a JSON encoded list of URLs that were parsed.
//
// If an URL lacks an attribute (e.g. `image`) then this attribute will be omitted from the result.
//
// Example:
//
//     ?content=Check+this+out+https://www.youtube.com/watch?v=dQw4w9WgXcQ
//
// Will return:
//
//     Type: "application/json"
//
// 	[
// 		{
// 			"title": "Rick Astley - Never Gonna Give You Up",
// 			"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
// 			"url_type": "video",
// 			"image": "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg"
// 		}
// 	]
//
// If handler was configured with FetchImageSize=true in its config, each hash
// may have additional fields `image_width` and `image_height` specifying
// dimensions of image provided by `image` attribute.
//
// Additionally you can supply `callback` to wrap the result in a JavaScript callback (JSONP),
// the type of this response would be "application/x-javascript"

package unfurlist

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"sync"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dyatlov/go-oembed/oembed"
)

// Configuration object for the HTTP handler
type UnfurlConfig struct {
	HTTPClient       *http.Client
	Log              *log.Logger
	OembedParser     *oembed.Oembed
	Cache            *memcache.Client
	MaxBodyChunkSize int64
	FetchImageSize   bool
}

const defaultMaxBodyChunkSize = 1024 * 64 //64KB

type unfurlHandler struct {
	Config *UnfurlConfig

	mu       sync.Mutex
	inFlight map[string]chan struct{} // in-flight urls processed
}

// Result that's returned back to the client
type unfurlResult struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Type        string `json:"url_type,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	ImageWidth  int    `json:"image_width,omitempty"`
	ImageHeight int    `json:"image_height,omitempty"`

	idx int
}

type unfurlResults []unfurlResult

func (rs unfurlResults) Len() int           { return len(rs) }
func (rs unfurlResults) Less(i, j int) bool { return rs[i].idx < rs[j].idx }
func (rs unfurlResults) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

// New returns new initialized unfurl handler. If config is nil, default values
// would be used.
func New(config *UnfurlConfig) http.Handler {
	var cfg *UnfurlConfig
	// copy config so that modifications to it won't leak to value provided
	// by caller
	if config == nil {
		cfg = new(UnfurlConfig)
	} else {
		tmp := *config
		cfg = &tmp
	}
	h := &unfurlHandler{
		Config:   cfg,
		inFlight: make(map[string]chan struct{}),
	}

	if h.Config.MaxBodyChunkSize == 0 {
		h.Config.MaxBodyChunkSize = defaultMaxBodyChunkSize
	}

	if h.Config.Log == nil {
		h.Config.Log = log.New(ioutil.Discard, "", 0)
	}

	if h.Config.OembedParser == nil {
		data, err := Asset("data/providers.json")
		if err != nil {
			panic(err)
		}
		oe := oembed.NewOembed()
		oe.ParseProviders(bytes.NewReader(data))
		h.Config.OembedParser = oe
	}

	return h
}

func (h *unfurlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()

	content := qs.Get("content")
	callback := qs.Get("callback")

	if content == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	urls := ParseURLs(content)

	jobResults := make(chan unfurlResult, 1)
	results := make(unfurlResults, 0, len(urls))
	abort := make(chan struct{}) // used to cancel background goroutines
	defer close(abort)

	for i, r := range urls {
		go h.processURL(i, r, jobResults, abort)
	}
	for i := 0; i < len(urls); i++ {
		select {
		case <-r.Cancel:
			return
		case res := <-jobResults:
			results = append(results, res)
		}
	}

	sort.Sort(results)

	if callback != "" {
		w.Header().Set("Content-Type", "application/x-javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}

	if callback != "" {
		io.WriteString(w, callback+"(")
		json.NewEncoder(w).Encode(results)
		w.Write([]byte(")"))
		return
	}
	json.NewEncoder(w).Encode(results)
}

// Processes the URL by first looking in cache, then trying oEmbed, OpenGraph
// If no match is found the result will be an object that just contains the URL
func (h *unfurlHandler) processURL(i int, url string, resp chan<- unfurlResult, abort <-chan struct{}) {
	waitLogged := false
	for {
		// spinlock-like loop to ensure we don't have two in-flight
		// outgoing requests for the same url
		h.mu.Lock()
		if ch, ok := h.inFlight[url]; ok {
			h.mu.Unlock()
			if !waitLogged {
				h.Config.Log.Printf("Wait for in-flight request to complete %q", url)
				waitLogged = true
			}
			<-ch // block until another goroutine processes the same url
		} else {
			ch = make(chan struct{})
			h.inFlight[url] = ch
			h.mu.Unlock()
			defer func() {
				h.mu.Lock()
				delete(h.inFlight, url)
				h.mu.Unlock()
				close(ch)
			}()
			break
		}
	}

	mc := h.Config.Cache

	if mc != nil {
		it, err := mc.Get(mcKey(url))
		if err == nil {
			var cached unfurlResult
			err = json.Unmarshal(it.Value, &cached)
			if err == nil {
				h.Config.Log.Printf("Cache hit for %q", url)
				select {
				case resp <- cached:
				case <-abort:
				}
				return
			}
		}
	}

	var result unfurlResult

	result = unfurlResult{idx: i}
	result.URL = url

	// Try oEmbed
	matched := OembedParseUrl(h, &result)

	if !matched {
		if htmlBody, err := h.fetchHTML(result.URL); err == nil {
			if matched = OpenGraphParseHTML(h, &result, htmlBody); !matched {
				matched = BasicParseParseHTML(h, &result, htmlBody)
			}
		}
	}

	switch absUrl, err := absoluteImageUrl(result.URL, result.Image); err {
	case errEmptyImageUrl:
	case nil:
		result.Image = absUrl
		if h.Config.FetchImageSize {
			if width, height, err := imageDimensions(result.Image, h.Config.HTTPClient); err != nil {
				h.Config.Log.Printf("dimensions detect for image %q: %v", result.Image, err)
			} else {
				result.ImageWidth, result.ImageHeight = width, height
			}
		}
	default:
		h.Config.Log.Printf("cannot get absolute image url for %q: %v", result.Image, err)
	}

	if matched && mc != nil {
		cdata, err := json.Marshal(result)
		if err == nil {
			h.Config.Log.Printf("Cache update for %q", url)
			mc.Set(&memcache.Item{Key: mcKey(url), Value: cdata})
		}
	}

	select {
	case resp <- result:
	case <-abort:
	}
}

// fetchHTML fetches the primary chunk of the document
// it does not care if the URL isn't HTML format
// the chunk size is determined by h.Config.MaxBodyChunkSize
func (h *unfurlHandler) fetchHTML(URL string) ([]byte, error) {
	client := h.Config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Get(URL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return nil, errors.New("bad status: " + response.Status)
	}

	firstChunk := io.LimitReader(response.Body, h.Config.MaxBodyChunkSize)

	return ioutil.ReadAll(firstChunk)
}

// mcKey returns string of hex representation of sha1 sum of string provided.
// Used to get safe keys to use with memcached
func mcKey(s string) string {
	h := sha1.New()
	io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}
