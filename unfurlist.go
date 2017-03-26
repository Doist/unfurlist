// Package unfurlist implements a service that unfurls URLs and provides more information about them.
//
// The current version supports Open Graph and oEmbed formats, Twitter card format is also planned.
// If the URL does not support common formats, unfurlist falls back to looking at common HTML tags
// such as <title> and <meta name="description">.
//
// The endpoint accepts GET and POST requests with `content` as the main argument.
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
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dyatlov/go-oembed/oembed"
)

const defaultMaxBodyChunkSize = 1024 * 64 //64KB
const userAgent = "unfurlist (https://github.com/Doist/unfurlist)"

type unfurlHandler struct {
	HTTPClient       *http.Client
	Log              Logger
	OembedParser     *oembed.Oembed
	Cache            *memcache.Client
	MaxBodyChunkSize int64
	FetchImageSize   bool

	// Headers specify key-value pairs of extra headers to add to each
	// outgoing request made by Handler. Headers length must be even,
	// otherwise Headers are ignored.
	Headers []string

	titleBlacklist []string

	pmap *prefixMap // built from BlacklistPrefix

	fetchers []FetchFunc
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

func (u *unfurlResult) Empty() bool {
	return u.URL == "" && u.Title == "" && u.Type == "" &&
		u.Description == "" && u.Image == ""
}

func (u *unfurlResult) normalize() {
	b := bytes.Join(bytes.Fields([]byte(u.Title)), []byte{' '})
	u.Title = string(b)
}

func (u *unfurlResult) Merge(u2 *unfurlResult) {
	if u2 == nil {
		return
	}
	if u.URL == "" {
		u.URL = u2.URL
	}
	if u.Title == "" {
		u.Title = u2.Title
	}
	if u.Type == "" {
		u.Type = u2.Type
	}
	if u.Description == "" {
		u.Description = u2.Description
	}
	if u.Image == "" {
		u.Image = u2.Image
	}
	if u.ImageWidth == 0 {
		u.ImageWidth = u2.ImageWidth
	}
	if u.ImageHeight == 0 {
		u.ImageHeight = u2.ImageHeight
	}
}

type unfurlResults []*unfurlResult

func (rs unfurlResults) Len() int           { return len(rs) }
func (rs unfurlResults) Less(i, j int) bool { return rs[i].idx < rs[j].idx }
func (rs unfurlResults) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

// ConfFunc is used to configure new unfurl handler; such functions should be
// used as arguments to New function
type ConfFunc func(*unfurlHandler) *unfurlHandler

// New returns new initialized unfurl handler. If no configuration functions
// provided, sane defaults would be used.
func New(conf ...ConfFunc) http.Handler {
	h := &unfurlHandler{
		inFlight: make(map[string]chan struct{}),
	}
	for _, f := range conf {
		h = f(h)
	}
	if h.HTTPClient == nil {
		h.HTTPClient = http.DefaultClient
	}
	if len(h.Headers)%2 != 0 {
		h.Headers = nil
	}
	if h.MaxBodyChunkSize == 0 {
		h.MaxBodyChunkSize = defaultMaxBodyChunkSize
	}
	if h.Log == nil {
		h.Log = log.New(ioutil.Discard, "", 0)
	}
	oe := oembed.NewOembed()
	_ = oe.ParseProviders(strings.NewReader(providersData))
	h.OembedParser = oe
	return h
}

func (h *unfurlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	content := r.Form.Get("content")
	callback := r.Form.Get("callback")

	if content == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	urls := parseURLsMax(content, 20)

	jobResults := make(chan *unfurlResult, 1)
	results := make(unfurlResults, 0, len(urls))
	ctx := r.Context()

	for i, r := range urls {
		go func(ctx context.Context, i int, link string, jobResults chan *unfurlResult) {
			select {
			case jobResults <- h.processURL(ctx, i, link):
			case <-ctx.Done():
			}
		}(ctx, i, r, jobResults)
	}
	for i := 0; i < len(urls); i++ {
		select {
		case <-ctx.Done():
			return
		case res := <-jobResults:
			results = append(results, res)
		}
	}

	sort.Sort(results)
	for _, r := range results {
		r.normalize()
	}

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
func (h *unfurlHandler) processURL(ctx context.Context, i int, link string) *unfurlResult {
	result := &unfurlResult{idx: i, URL: link}
	waitLogged := false
	for {
		// spinlock-like loop to ensure we don't have two in-flight
		// outgoing requests for the same link
		h.mu.Lock()
		if ch, ok := h.inFlight[link]; ok {
			h.mu.Unlock()
			if !waitLogged {
				h.Log.Printf("Wait for in-flight request to complete %q", link)
				waitLogged = true
			}
			select {
			case <-ch: // block until another goroutine processes the same url
			case <-ctx.Done():
				return result
			}
		} else {
			ch = make(chan struct{})
			h.inFlight[link] = ch
			h.mu.Unlock()
			defer func() {
				h.mu.Lock()
				delete(h.inFlight, link)
				h.mu.Unlock()
				close(ch)
			}()
			break
		}
	}

	if h.pmap != nil && h.pmap.Match(link) { // blacklisted
		h.Log.Printf("Blacklisted %q", link)
		return result
	}

	if mc := h.Cache; mc != nil {
		if it, err := mc.Get(mcKey(link)); err == nil {
			var cached unfurlResult
			if err = json.Unmarshal(it.Value, &cached); err == nil {
				h.Log.Printf("Cache hit for %q", link)
				cached.idx = i
				return &cached
			}
		}
	}
	chunk, err := h.fetchData(ctx, result.URL)
	if err != nil {
		return result
	}
	for _, f := range h.fetchers {
		meta, ok := f(chunk.url)
		if !ok || !meta.Valid() {
			continue
		}
		result.Title = meta.Title
		result.Type = meta.Type
		result.Description = meta.Description
		result.Image = meta.Image
		result.ImageWidth = meta.ImageWidth
		result.ImageHeight = meta.ImageHeight
		goto hasMatch
	}

	if res := oembedParseURL(h, chunk.url.String()); res != nil {
		result.Merge(res)
		goto hasMatch
	}
	if res, err := oembedDiscoverable(ctx, h.HTTPClient, chunk); err == nil && res != nil {
		result.Merge(res)
		goto hasMatch
	}
	for _, f := range []func(*pageChunk) *unfurlResult{
		openGraphParseHTML,
		basicParseHTML,
	} {
		if res := f(chunk); res != nil {
			if h.titleBlacklist != nil && res.Title != "" {
				lt := strings.ToLower(res.Title)
				for _, s := range h.titleBlacklist {
					if strings.Contains(lt, s) {
						goto hasMatch
					}
				}
			}
			result.Merge(res)
			goto hasMatch
		}
	}

hasMatch:
	switch absURL, err := absoluteImageURL(result.URL, result.Image); err {
	case errEmptyImageURL:
	case nil:
		result.Image = absURL
		if h.FetchImageSize && (result.ImageWidth == 0 || result.ImageHeight == 0) {
			if width, height, err := imageDimensions(ctx, h.HTTPClient, result.Image); err != nil {
				h.Log.Printf("dimensions detect for image %q: %v", result.Image, err)
			} else {
				result.ImageWidth, result.ImageHeight = width, height
			}
		}
	default:
		h.Log.Printf("cannot get absolute image url for %q: %v", result.Image, err)
		result.Image, result.ImageWidth, result.ImageHeight = "", 0, 0
	}

	if mc := h.Cache; mc != nil && !result.Empty() {
		if cdata, err := json.Marshal(result); err == nil {
			h.Log.Printf("Cache update for %q", link)
			mc.Set(&memcache.Item{Key: mcKey(link), Value: cdata})
		}
	}
	return result
}

// pageChunk describes first chunk of resource
type pageChunk struct {
	data []byte   // first chunk of resource data
	url  *url.URL // final url resource was fetched from (after all redirects)
	ct   string   // Content-Type as reported by server
}

// fetchData fetches the first chunk of the resource. The chunk size is
// determined by h.MaxBodyChunkSize.
func (h *unfurlHandler) fetchData(ctx context.Context, URL string) (*pageChunk, error) {
	client := h.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for i := 0; i < len(h.Headers); i += 2 {
		req.Header.Set(h.Headers[i], h.Headers[i+1])
	}
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.New("bad status: " + resp.Status)
	}
	head, err := ioutil.ReadAll(io.LimitReader(resp.Body, h.MaxBodyChunkSize))
	if err != nil {
		return nil, err
	}
	return &pageChunk{
		data: head,
		url:  resp.Request.URL,
		ct:   resp.Header.Get("Content-Type"),
	}, nil
}

// mcKey returns string of hex representation of sha1 sum of string provided.
// Used to get safe keys to use with memcached
func mcKey(s string) string {
	h := sha1.New()
	io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

//go:generate go run assets-update.go
