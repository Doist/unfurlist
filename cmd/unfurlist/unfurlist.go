// Command unfurlist implements http server exposing API endpoint
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/Doist/unfurlist"
	"github.com/Doist/unfurlist/internal/useragent"
	"github.com/artyom/autoflags"
	"github.com/artyom/oembed"
	"github.com/bradfitz/gomemcache/memcache"
)

func main() {
	args := struct {
		Listen          string        `flag:"listen,address to listen, set both -sslcert and -sslkey for HTTPS"`
		Pprof           string        `flag:"pprof,address to serve pprof data"`
		Cert            string        `flag:"sslcert,path to certificate file (PEM format)"`
		Key             string        `flag:"sslkey,path to certificate file (PEM format)"`
		Cache           string        `flag:"cache,address of memcached, disabled if empty"`
		Blocklist       string        `flag:"blocklist,file with url prefixes to block, one per line"`
		WithDimensions  bool          `flag:"withDimensions,return image dimensions if possible (extra request to fetch image)"`
		Timeout         time.Duration `flag:"timeout,timeout for remote i/o"`
		GoogleMapsKey   string        `flag:"googlemapskey,Google Static Maps API key to generate map previews"`
		VideoDomains    string        `flag:"videoDomains,comma-separated list of domains that host video+thumbnails"`
		ImageProxyURL   string        `flag:"image.proxy.url,url to proxy http:// image urls through"`
		ImageProxyKey   string        `flag:"image.proxy.secret,secret to generate sha1 HMAC signatures"`
		MaxResults      int           `flag:"max,maximum number of results to get for single request"`
		Ping            bool          `flag:"ping,respond with 200 OK on /ping path (for health checks)"`
		OembedProviders string        `flag:"oembedProviders,custom oembed providers list in json format"`
	}{
		Listen:     "localhost:8080",
		Pprof:      "localhost:6060",
		Timeout:    30 * time.Second,
		MaxResults: unfurlist.DefaultMaxResults,
	}
	flag.StringVar(&args.Blocklist, "blacklist", args.Blocklist, "DEPRECATED: use -blocklist instead")
	autoflags.Define(&args)
	flag.Parse()

	if args.Timeout < 0 {
		args.Timeout = 0
	}
	httpClient := &http.Client{
		CheckRedirect: failOnLoginPages,
		Timeout:       args.Timeout,
		Transport: useragent.Set(&http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}, "unfurlist (https://github.com/Doist/unfurlist)"),
	}
	logFlags := log.LstdFlags
	if os.Getenv("AWS_EXECUTION_ENV") != "" {
		logFlags = 0
	}
	configs := []unfurlist.ConfFunc{
		unfurlist.WithExtraHeaders(map[string]string{
			"Accept-Language": "en;q=1, *;q=0.5",
		}),
		unfurlist.WithLogger(log.New(os.Stderr, "", logFlags)),
		unfurlist.WithHTTPClient(httpClient),
		unfurlist.WithImageDimensions(args.WithDimensions),
		unfurlist.WithBlocklistTitles(titleBlocklist),
		unfurlist.WithMaxResults(args.MaxResults),
	}
	if args.OembedProviders != "" {
		data, err := os.ReadFile(args.OembedProviders)
		if err != nil {
			log.Fatal(err)
		}
		fn, err := oembed.Providers(bytes.NewReader(data))
		if err != nil {
			log.Fatal(err)
		}
		configs = append(configs, unfurlist.WithOembedLookupFunc(fn))
	}
	if args.Blocklist != "" {
		prefixes, err := readBlocklist(args.Blocklist)
		if err != nil {
			log.Fatal(err)
		}
		configs = append(configs, unfurlist.WithBlocklistPrefixes(prefixes))
	}
	if args.Cache != "" {
		log.Print("Enable cache at ", args.Cache)
		configs = append(configs, unfurlist.WithMemcache(memcache.New(args.Cache)))
	}
	if args.ImageProxyURL != "" {
		configs = append(configs,
			unfurlist.WithImageProxy(args.ImageProxyURL, args.ImageProxyKey))
	}
	var ff []unfurlist.FetchFunc
	if args.GoogleMapsKey != "" {
		ff = append(ff, unfurlist.GoogleMapsFetcher(args.GoogleMapsKey))
	}
	if args.VideoDomains != "" {
		ff = append(ff, videoThumbnailsFetcher(strings.Split(args.VideoDomains, ",")...))
	}
	if ff != nil {
		configs = append(configs, unfurlist.WithFetchers(ff...))
	}

	handler := unfurlist.New(configs...)
	if args.Pprof != "" {
		go func(addr string) { log.Println(http.ListenAndServe(addr, nil)) }(args.Pprof)
	}
	go func() {
		// on a highly used system unfurlist can accumulate a lot of
		// idle connections occupying memory; force periodic close of
		// them
		for range time.NewTicker(2 * time.Minute).C {
			if c, ok := httpClient.Transport.(interface {
				CloseIdleConnections()
			}); ok {
				c.CloseIdleConnections()
			}
		}
	}()
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	if args.Ping {
		mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}
	srv := &http.Server{
		Addr:         args.Listen,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  30 * time.Second,
		Handler:      mux,
	}
	if args.Cert != "" && args.Key != "" {
		log.Fatal(srv.ListenAndServeTLS(args.Cert, args.Key))
	} else {
		log.Fatal(srv.ListenAndServe())
	}
}

func readBlocklist(blocklist string) ([]string, error) {
	f, err := os.Open(blocklist)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(io.LimitReader(f, 512*1024))
	var prefixes []string
	for s.Scan() {
		if bytes.HasPrefix(s.Bytes(), []byte("http")) {
			prefixes = append(prefixes, s.Text())
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return prefixes, nil
}

// failOnLoginPages can be used as http.Client.CheckRedirect to skip redirects
// to login pages of most commonly used services or most commonly named login
// pages. It also checks depth of redirect chain and stops on more then 10
// consecutive redirects.
func failOnLoginPages(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if l := len(via); l > 0 && *req.URL == *via[l-1].URL {
		return errors.New("redirect loop")
	}
	if strings.Contains(strings.ToLower(req.URL.Host), "login") ||
		loginPathRe.MatchString(req.URL.Path) {
		return errWantLogin
	}
	u := *req.URL
	u.RawQuery, u.Fragment = "", ""
	if _, ok := loginPages[(&u).String()]; ok {
		return errWantLogin
	}
	return nil
}

var loginPathRe = regexp.MustCompile(`(?i)login|sign.?in`)

var errWantLogin = errors.New("resource requires login")

// loginPages is a set of popular services' known login pages
var loginPages map[string]struct{}

func init() {
	pages := []string{
		"https://bitbucket.org/account/signin/",
	}
	loginPages = make(map[string]struct{}, len(pages))
	for _, u := range pages {
		loginPages[u] = struct{}{}
	}

}

var titleBlocklist = []string{
	"robot check", // Amazon
}

// videoThumbnailsFetcher return unfurlist.FetchFunc that returns metadata
// with url to video thumbnail file for supported domains.
func videoThumbnailsFetcher(domains ...string) func(context.Context, *http.Client, *url.URL) (*unfurlist.Metadata, bool) {
	doms := make(map[string]struct{})
	for _, d := range domains {
		doms[d] = struct{}{}
	}
	return func(_ context.Context, _ *http.Client, u *url.URL) (*unfurlist.Metadata, bool) {
		if _, ok := doms[u.Host]; !ok {
			return nil, false
		}
		switch strings.ToLower(path.Ext(u.Path)) {
		default:
			return nil, false
		case ".mp4", ".mov", ".m4v", ".3gp", ".webm", ".mkv":
		}
		u2 := &url.URL{
			Scheme: u.Scheme,
			Host:   u.Host,
			Path:   u.Path + ".thumb",
		}
		return &unfurlist.Metadata{
			Title: path.Base(u.Path),
			Type:  "video",
			Image: u2.String(),
		}, true
	}
}
