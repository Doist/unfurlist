// Command unfurlist implements http server exposing API endpoint
package main

import (
	"bufio"
	"bytes"
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
	"strings"
	"time"

	"github.com/Doist/unfurlist"
	"github.com/artyom/autoflags"
	"github.com/artyom/useragent"
	"github.com/bradfitz/gomemcache/memcache"
)

func main() {
	args := struct {
		Listen         string        `flag:"listen,address to listen, set both -sslcert and -sslkey for HTTPS"`
		Pprof          string        `flag:"pprof,address to serve pprof data"`
		Cert           string        `flag:"sslcert,path to certificate file (PEM format)"`
		Key            string        `flag:"sslkey,path to certificate file (PEM format)"`
		Cache          string        `flag:"cache,address of memcached, disabled if empty"`
		Blacklist      string        `flag:"blacklist,file with url prefixes to blacklist, one per line"`
		WithDimensions bool          `flag:"withDimensions,return image dimensions if possible (extra request to fetch image)"`
		Timeout        time.Duration `flag:"timeout,timeout for remote i/o"`
		GoogleMapsKey  string        `flag:"googlemapskey,Google Static Maps API key to generate map previews"`
		VideoDomains   string        `flag:"videoDomains,comma-separated list of domains that host video+thumbnails"`
	}{
		Listen:  "localhost:8080",
		Pprof:   "localhost:6060",
		Timeout: 30 * time.Second,
	}
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
	configs := []unfurlist.ConfFunc{
		unfurlist.WithExtraHeaders(map[string]string{
			"Accept-Language": "en;q=1, *;q=0.5",
		}),
		unfurlist.WithLogger(log.New(os.Stderr, "", log.LstdFlags)),
		unfurlist.WithHTTPClient(httpClient),
		unfurlist.WithImageDimensions(args.WithDimensions),
		unfurlist.WithBlacklistTitles(titleBlacklist),
	}
	if args.Blacklist != "" {
		prefixes, err := readBlacklist(args.Blacklist)
		if err != nil {
			log.Fatal(err)
		}
		configs = append(configs, unfurlist.WithBlacklistPrefixes(prefixes))
	}
	if args.Cache != "" {
		log.Print("Enable cache at ", args.Cache)
		configs = append(configs, unfurlist.WithMemcache(memcache.New(args.Cache)))
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

	srv := &http.Server{
		Addr:         args.Listen,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  30 * time.Second,
		Handler:      handler,
	}
	if args.Cert != "" && args.Key != "" {
		log.Fatal(srv.ListenAndServeTLS(args.Cert, args.Key))
	} else {
		log.Fatal(srv.ListenAndServe())
	}
}

func readBlacklist(blacklist string) ([]string, error) {
	f, err := os.Open(blacklist)
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

func readSubnets(name string) ([]*net.IPNet, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var subnets []*net.IPNet
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if b := scanner.Bytes(); len(b) == 0 || b[0] == '#' {
			continue
		}
		_, n, err := net.ParseCIDR(scanner.Text())
		if err != nil {
			return nil, err
		}
		subnets = append(subnets, n)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return subnets, nil
}

// failOnLoginPages can be used as http.Client.CheckRedirect to skip redirects
// to login pages of most commonly used services or most commonly named login
// pages. It also checks depth of redirect chain and stops on more then 10
// consecutive redirects.
func failOnLoginPages(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if strings.Contains(strings.ToLower(req.URL.Host), "login") ||
		strings.Contains(strings.ToLower(req.URL.Path), "login") {
		return errWantLogin
	}
	u := *req.URL
	u.RawQuery, u.Fragment = "", ""
	if _, ok := loginPages[(&u).String()]; ok {
		return errWantLogin
	}
	return nil
}

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

var titleBlacklist = []string{
	"robot check", // Amazon
}

// videoThumbnailsFetcher return unfurlist.FetchFunc that returns metadata
// with url to video thumbnail file for supported domains.
func videoThumbnailsFetcher(domains ...string) func(*url.URL) (*unfurlist.Metadata, bool) {
	doms := make(map[string]struct{})
	for _, d := range domains {
		doms[d] = struct{}{}
	}
	return func(u *url.URL) (*unfurlist.Metadata, bool) {
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
