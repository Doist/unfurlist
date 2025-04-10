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
//	?content=Check+this+out+https://www.youtube.com/watch?v=dQw4w9WgXcQ
//
// Will return:
//
//	    Type: "application/json"
//
//		[
//			{
//				"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
//				"title": "Rick Astley - Never Gonna Give You Up (Video)",
//				"url_type": "video.other",
//				"description": "Rick Astley - Never Gonna Give You Up...",
//				"site_name": "YouTube",
//				"favicon": "https://www.youtube.com/yts/img/favicon_32-vflOogEID.png",
//				"image": "https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg"
//			}
//		]
//
// If handler was configured with FetchImageSize=true in its config, each hash
// may have additional fields `image_width` and `image_height` specifying
// dimensions of image provided by `image` attribute.
//
// Additionally you can supply `callback` to wrap the result in a JavaScript callback (JSONP),
// the type of this response would be "application/x-javascript"
//
// If an optional `markdown` boolean argument is set (markdown=true), then
// provided content is parsed as markdown formatted text and links are extracted
// in context-aware mode — i.e. preformatted text blocks are skipped.
//
// # Security
//
// Care should be taken when running this inside internal network since it may
// disclose internal endpoints. It is a good idea to run the service on
// a separate host in an isolated subnet.
//
// Alternatively access to internal resources may be limited with firewall
// rules, i.e. if service is running as 'unfurlist' user on linux box, the
// following iptables rules can reduce chances of it connecting to internal
// endpoints (note this example is for ipv4 only!):
//
//	iptables -A OUTPUT -m owner --uid-owner unfurlist -p tcp --syn \
//		-d 127/8,10/8,169.254/16,172.16/12,192.168/16 \
//		-j REJECT --reject-with icmp-net-prohibited
//	ip6tables -A OUTPUT -m owner --uid-owner unfurlist -p tcp --syn \
//		-d ::1/128,fe80::/10 \
//		-j REJECT --reject-with adm-prohibited
package unfurlist

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/sync/singleflight"

	"github.com/artyom/httpflags"
	"github.com/artyom/oembed"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/golang/snappy"
)

const defaultMaxBodyChunkSize = 1024 * 64 //64KB

// DefaultMaxResults is maximum number of urls to process if not configured by
// WithMaxResults function
const DefaultMaxResults = 20

type unfurlHandler struct {
	HTTPClient       *http.Client
	Log              Logger
	oembedLookupFunc oembed.LookupFunc
	Cache            *memcache.Client
	MaxBodyChunkSize int64
	FetchImageSize   bool

	// Headers specify key-value pairs of extra headers to add to each
	// outgoing request made by Handler. Headers length must be even,
	// otherwise Headers are ignored.
	Headers []string

	titleBlocklist []string

	pmap *prefixMap // built from BlocklistPrefix

	maxResults int // max number of urls to process

	fetchers []FetchFunc
	inFlight singleflight.Group // in-flight urls processed
}

// Result that's returned back to the client
type unfurlResult struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Type        string `json:"url_type,omitempty"`
	Description string `json:"description,omitempty"`
	HTML        string `json:"html,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	Favicon     string `json:"favicon,omitempty"`
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
	if u.HTML == "" {
		u.HTML = u2.HTML
	}
	if u.SiteName == "" {
		u.SiteName = u2.SiteName
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

// ConfFunc is used to configure new unfurl handler; such functions should be
// used as arguments to New function
type ConfFunc func(*unfurlHandler) *unfurlHandler

// New returns new initialized unfurl handler. If no configuration functions
// provided, sane defaults would be used.
func New(conf ...ConfFunc) http.Handler {
	h := &unfurlHandler{
		maxResults: DefaultMaxResults,
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
		h.Log = log.New(io.Discard, "", 0)
	}
	if h.oembedLookupFunc == nil {
		fn, err := oembed.Providers(bytes.NewReader(providersData))
		if err != nil {
			panic(err)
		}
		h.oembedLookupFunc = fn
	}
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
	args := struct {
		Content  string `flag:"content"`
		Callback string `flag:"callback"`
		Markdown bool   `flag:"markdown"`
	}{}
	if err := httpflags.Parse(&args, r); err != nil || args.Content == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var urls []string
	switch {
	case args.Markdown:
		urls = parseMarkdownURLs(args.Content, h.maxResults)
	default:
		urls = parseURLsMax(args.Content, h.maxResults)
	}

	jobResults := make(chan *unfurlResult, 1)
	results := make([]*unfurlResult, 0, len(urls))
	ctx := r.Context()

	for i, r := range urls {
		go func(ctx context.Context, i int, link string, jobResults chan *unfurlResult) {
			select {
			case jobResults <- h.processURLidx(ctx, i, link):
			case <-ctx.Done():
			}
		}(ctx, i, r, jobResults)
	}
	for range urls {
		select {
		case <-ctx.Done():
			return
		case res := <-jobResults:
			results = append(results, res)
		}
	}

	slices.SortFunc(results, func(a, b *unfurlResult) int { return cmp.Compare(a.idx, b.idx) })
	for _, r := range results {
		r.normalize()
	}

	if args.Callback != "" {
		w.Header().Set("Content-Type", "application/x-javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}

	if args.Callback != "" {
		io.WriteString(w, args.Callback+"(")
		json.NewEncoder(w).Encode(results)
		w.Write([]byte(")"))
		return
	}
	json.NewEncoder(w).Encode(results)
}

// processURLidx wraps processURL and adds provided index i to the result. It
// also collapses multiple in-flight requests for the same url to a single
// processURL call
func (h *unfurlHandler) processURLidx(ctx context.Context, i int, link string) *unfurlResult {
	defer h.inFlight.Forget(link)
	v, _, shared := h.inFlight.Do(link, func() (any, error) { return h.processURL(ctx, link), nil })
	res, ok := v.(*unfurlResult)
	if !ok {
		panic("got unexpected type from singleflight.Do")
	}
	if shared && (*res == unfurlResult{URL: link}) && ctx.Err() == nil {
		// an *incomplete* shared result, e.g. if context in another goroutine
		// that called processURL was canceled early, need to refetch
		res = h.processURL(ctx, link)
	}
	res2 := *res // make a copy because we're going to modify it
	res2.idx = i
	return &res2
}

// Processes the URL by first looking in cache, then trying oEmbed, OpenGraph
// If no match is found the result will be an object that just contains the URL
func (h *unfurlHandler) processURL(ctx context.Context, link string) *unfurlResult {
	result := &unfurlResult{URL: link}
	if h.pmap != nil && h.pmap.Match(link) { // blocklisted
		h.Log.Printf("Blocklisted %q", link)
		return result
	}

	if mc := h.Cache; mc != nil {
		if it, err := mc.Get(mcKey(link)); err == nil {
			if b, err := snappy.Decode(nil, it.Value); err == nil {
				var cached unfurlResult
				if err = json.Unmarshal(b, &cached); err == nil {
					h.Log.Printf("Cache hit for %q", link)
					return &cached
				}
			}
		}
	}
	var chunk *pageChunk
	var err error
	// Optimistically apply oembed logic to url we have, which can only work
	// for non-minimized urls; however if it works, it'll let us skip fetching
	// url altogether. This can also somewhat help against sites redirecting to
	// captchas/login pages when they see requests from non "home ISP"
	// networks.
	if endpoint, ok := h.oembedLookupFunc(result.URL); ok {
		if res, err := fetchOembed(ctx, endpoint, h.httpGet); err == nil {
			result.Merge(res)
			goto hasMatch
		}
	}
	chunk, err = h.fetchData(ctx, result.URL)
	if err != nil {
		if chunk != nil && strings.Contains(chunk.url.Host, "youtube.com") {
			if meta, ok := youtubeFetcher(ctx, h.HTTPClient, chunk.url); ok && meta.Valid() {
				result.Title = meta.Title
				result.Type = meta.Type
				result.Description = meta.Description
				result.Image = meta.Image
				result.ImageWidth = meta.ImageWidth
				result.ImageHeight = meta.ImageHeight
				goto hasMatch
			}
		}
		return result
	}
	if s, err := h.faviconLookup(ctx, chunk); err == nil && s != "" {
		result.Favicon = s
	}
	for _, f := range h.fetchers {
		meta, ok := f(ctx, h.HTTPClient, chunk.url)
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

	if res := openGraphParseHTML(chunk); res != nil {
		if !blocklisted(h.titleBlocklist, res.Title) {
			result.Merge(res)
			goto hasMatch
		}
	}
	if endpoint, found := chunk.oembedEndpoint(h.oembedLookupFunc); found {
		if res, err := fetchOembed(ctx, endpoint, h.httpGet); err == nil {
			result.Merge(res)
			goto hasMatch
		}
	}
	if res := basicParseHTML(chunk); res != nil {
		if !blocklisted(h.titleBlocklist, res.Title) {
			result.Merge(res)
		}
	}

hasMatch:
	switch absURL, err := absoluteImageURL(result.URL, result.Image); err {
	case errEmptyImageURL:
	case nil:
		switch {
		case validURL(absURL):
			result.Image = absURL
		default:
			result.Image = ""
		}
		if result.Image != "" && h.FetchImageSize && (result.ImageWidth == 0 || result.ImageHeight == 0) {
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
			mc.Set(&memcache.Item{Key: mcKey(link), Value: snappy.Encode(nil, cdata)})
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

func (p *pageChunk) oembedEndpoint(fn oembed.LookupFunc) (url string, found bool) {
	if p == nil || fn == nil {
		return "", false
	}
	if u, ok := fn(p.url.String()); ok {
		return u, true
	}
	r, err := charset.NewReader(bytes.NewReader(p.data), p.ct)
	if err != nil {
		return "", false
	}
	if u, ok, err := oembed.Discover(r); err == nil && ok {
		return u, true
	}
	return "", false
}

func (h *unfurlHandler) httpGet(ctx context.Context, URL string) (*http.Response, error) {
	client := h.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(h.Headers); i += 2 {
		req.Header.Set(h.Headers[i], h.Headers[i+1])
	}
	req = req.WithContext(ctx)
	return client.Do(req)
}

// fetchData fetches the first chunk of the resource. The chunk size is
// determined by h.MaxBodyChunkSize.
func (h *unfurlHandler) fetchData(ctx context.Context, URL string) (*pageChunk, error) {
	resp, err := h.httpGet(ctx, URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		// returning pageChunk with the final url (after all redirects) so that
		// special cases like youtube returning 429 can be handled by
		// specialized fetchers like youtubeFetcher
		return &pageChunk{url: resp.Request.URL}, errors.New("bad status: " + resp.Status)
	}
	if resp.Header.Get("Content-Encoding") == "deflate" &&
		(strings.HasSuffix(resp.Request.Host, "twitter.com") ||
			strings.HasSuffix(resp.Request.Host, "x.com")) {
		// twitter/X sends unsolicited deflate-encoded responses
		// violating RFC; workaround this.
		// See https://golang.org/issues/18779 for background
		var err error
		if resp.Body, err = zlib.NewReader(resp.Body); err != nil {
			return nil, err
		}
	}
	head, err := io.ReadAll(io.LimitReader(resp.Body, h.MaxBodyChunkSize))
	if err != nil {
		return nil, err
	}
	return &pageChunk{
		data: head,
		url:  resp.Request.URL,
		ct:   resp.Header.Get("Content-Type"),
	}, nil
}

func (h *unfurlHandler) faviconLookup(ctx context.Context, chunk *pageChunk) (string, error) {
	if strings.HasPrefix(chunk.ct, "text/html") {
		href := extractFaviconLink(chunk.data, chunk.ct)
		if href == "" {
			goto probeDefaultIcon
		}
		u, err := url.Parse(href)
		if err != nil {
			return "", err
		}
		return chunk.url.ResolveReference(u).String(), nil
	}
probeDefaultIcon:
	u := &url.URL{Scheme: chunk.url.Scheme, Host: chunk.url.Host, Path: "/favicon.ico"}
	client := h.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return "", err
	}
	for i := 0; i < len(h.Headers); i += 2 {
		req.Header.Set(h.Headers[i], h.Headers[i+1])
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	r, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusOK {
		return u.String(), nil
	}
	return "", nil
}

// mcKey returns string of hex representation of sha1 sum of string provided.
// Used to get safe keys to use with memcached
func mcKey(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func blocklisted(blocklilst []string, title string) bool {
	if title == "" || len(blocklilst) == 0 {
		return false
	}
	lt := strings.ToLower(title)
	for _, s := range blocklilst {
		if strings.Contains(lt, s) {
			return true
		}
	}
	return false
}

//go:embed data/providers.json
var providersData []byte
