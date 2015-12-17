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
// Additionally you can supply `callback` to wrap the result in a JavaScript callback (JSONP),
// the type of this response would be "application/x-javascript"

package unfurlist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"

	"github.com/dyatlov/go-oembed/oembed"
	"github.com/rainycape/memcache"
)

// Configuration object for the HTTP handler
type UnfurlConfig struct {
	Log               *log.Logger
	OembedParser      *oembed.Oembed
	Cache             *memcache.Client
	MaxBodyChunckSize int64
}

const defaultMaxBodyChunckSize = 1024 * 64 //64KB

type unfurlHandler struct {
	Config *UnfurlConfig
}

// Result that's returned back to the client
type unfurlResult struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Type        string `json:"url_type,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`

	idx int
}

type serviceResult struct {
	HasMatch bool
	Result   *unfurlResult
}

type unfurlResults []unfurlResult

func (rs unfurlResults) Len() int           { return len(rs) }
func (rs unfurlResults) Less(i, j int) bool { return rs[i].idx < rs[j].idx }
func (rs unfurlResults) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

func New(config *UnfurlConfig) http.Handler {
	h := &unfurlHandler{
		Config: config,
	}

	if h.Config.MaxBodyChunckSize == 0 {
		h.Config.MaxBodyChunckSize = defaultMaxBodyChunckSize
	}

	if h.Config.Log == nil {
		h.Config.Log = log.New(ioutil.Discard, "", 0)
	}

	// Oembed
	data, err := Asset("data/providers.json")
	if err != nil {
		panic(err)
	}
	oe := oembed.NewOembed()
	oe.ParseProviders(bytes.NewReader(data))
	h.Config.OembedParser = oe

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

	for i, r := range urls {
		go h.processURL(i, r, jobResults)
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

	fmt.Fprint(w, h.toJSON(results, callback))
}

// Processes the URL by first looking in cache, then trying oEmbed, OpenGraph
// If no match is found the result will be an object that just contains the URL
func (h *unfurlHandler) processURL(i int, url string, resp chan<- unfurlResult) {
	mc := h.Config.Cache

	if mc != nil {
		it, err := mc.Get(url)
		if err == nil {
			var cached unfurlResult
			err = json.Unmarshal(it.Value, &cached)
			if err == nil {
				h.Config.Log.Print("Hitting cache")
				resp <- cached
				return
			}
		}
	}

	var result unfurlResult

	result = unfurlResult{idx: i}
	result.URL = url

	// Try oEmbed
	serviceResult := OembedParseUrl(h, &result)

	// Parse the HTML
	htmlBody, err := h.fetchHTML(result.URL)
	if err != nil {
		resp <- result
		return
	}

	// Try OpenGraph
	if !serviceResult.HasMatch {
		serviceResult = OpenGraphParseHTML(h, &result, htmlBody)
	}

	// Fallback to parsing basic HTML
	if !serviceResult.HasMatch {
		serviceResult = BasicParseParseHTML(h, &result, htmlBody)
	}

	if mc != nil {
		cdata, err := json.Marshal(result)
		if err == nil {
			h.Config.Log.Print("Updating URL with cache")
			mc.Set(&memcache.Item{Key: url, Value: cdata})
		}
	}

	resp <- result
}

// toJSON converts the set of messages to a JSON-encoded string
func (h *unfurlHandler) toJSON(messages unfurlResults, callback string) string {
	jsonMessages, _ := json.Marshal(messages)
	messagesStr := string(jsonMessages)
	if callback != "" {
		messagesStr = fmt.Sprintf(`%s(%s)`, callback, messagesStr)
	}
	return messagesStr
}

// fetchHTML fetches the primary chunk of the document
// it does not care if the URL isn't HTML format
// the chunk size is determined by h.Config.MaxBodyChunckSize
func (h *unfurlHandler) fetchHTML(URL string) (string, error) {
	response, err := http.Get(URL)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	firstChunk := io.LimitReader(response.Body, h.Config.MaxBodyChunckSize)

	body, err := ioutil.ReadAll(firstChunk)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
