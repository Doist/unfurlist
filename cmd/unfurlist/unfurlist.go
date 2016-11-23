// Command unfurlist implements http server exposing API endpoint
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/Doist/unfurlist"
	"github.com/bradfitz/gomemcache/memcache"
)

func main() {
	var (
		listen            = "127.0.0.1:8080"
		pprofListen       = "127.0.0.1:6060"
		cache             = ""
		certfile, keyfile string
		blacklist         string
		privateSubnets    string
		timeout           = 30 * time.Second
		withDimensions    bool
		globalOnly        bool
		googleMapsKey     string
	)
	flag.DurationVar(&timeout, "timeout", timeout, "timeout for remote i/o")
	flag.StringVar(&googleMapsKey, "googlemapskey", googleMapsKey, "Google Static Maps API key to generate map previews")
	flag.StringVar(&listen, "listen", listen, "`address` to listen, set both -sslcert and -sslkey for HTTPS")
	flag.StringVar(&pprofListen, "pprof", pprofListen, "`address` to serve pprof data at (usually localhost)")
	flag.StringVar(&certfile, "sslcert", "", "path to certificate `file` (PEM)")
	flag.StringVar(&keyfile, "sslkey", "", "path to certificate key `file` (PEM)")
	flag.StringVar(&cache, "cache", cache, "`address` of memcached, if unset, caching is not used")
	flag.StringVar(&blacklist, "blacklist", blacklist, "path to `file` with url prefixes to blacklist, one per line")
	flag.StringVar(&privateSubnets, "privateSubnets", privateSubnets, "path to `file` with subnets (CIDR) to block requests to, one per line")
	flag.BoolVar(&withDimensions, "withDimensions", withDimensions, "return image dimensions in result where possible (extra external request to fetch image)")
	flag.BoolVar(&globalOnly, "globalOnly", globalOnly, "allow only connections to global unicast IPs")
	flag.Parse()

	if timeout < 0 {
		timeout = 0
	}
	config := unfurlist.Config{
		HTTPClient: &http.Client{
			CheckRedirect: failOnLoginPages,
			Transport:     http.DefaultTransport,
			Timeout:       timeout,
		},
		Headers:        []string{"Accept-Language", "en;q=1, *;q=0.5"},
		Log:            log.New(os.Stderr, "", log.LstdFlags),
		FetchImageSize: withDimensions,
	}
	switch {
	case privateSubnets != "":
		sn, err := readSubnets(privateSubnets)
		if err != nil {
			log.Fatal(err)
		}
		tr, err := restrictedTransport(sn, globalOnly)
		if err != nil {
			log.Fatal(err)
		}
		config.HTTPClient.Transport = tr
	case privateSubnets == "" && globalOnly:
		tr, err := restrictedTransport(nil, globalOnly)
		if err != nil {
			log.Fatal(err)
		}
		config.HTTPClient.Transport = tr
	}
	if blacklist != "" {
		prefixes, err := readBlacklist(blacklist)
		if err != nil {
			log.Fatal(err)
		}
		config.BlacklistPrefix = prefixes
	}
	if cache != "" {
		log.Print("Enable cache at ", cache)
		config.Cache = memcache.New(cache)
	}

	var handler http.Handler
	switch googleMapsKey {
	case "":
		handler = unfurlist.New(&config)
	default:
		handler = unfurlist.WithFetchers(unfurlist.New(&config),
			unfurlist.GoogleMapsFetcher(googleMapsKey))
	}
	if pprofListen != "" {
		go func(addr string) { log.Println(http.ListenAndServe(addr, nil)) }(pprofListen)
	}
	go func() {
		// on a highly used system unfurlist can accumulate a lot of
		// idle connections occupying memory; force periodic close of
		// them
		for range time.NewTicker(2 * time.Minute).C {
			config.HTTPClient.Transport.(*http.Transport).CloseIdleConnections()
		}
	}()

	if certfile != "" && keyfile != "" {
		log.Fatal(http.ListenAndServeTLS(listen, certfile, keyfile, handler))
	} else {
		log.Fatal(http.ListenAndServe(listen, handler))
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

// restrictedTransport returns http.Transport that blocks attempts to connect to specified
// private subnets, if globalOnly specified, connections only allowed to IPs for which
// IsGlobalUnicast() is true.
func restrictedTransport(privateSubnets []*net.IPNet, globalOnly bool) (http.RoundTripper, error) {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	dialFunc := func(network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if globalOnly && !ip.IsGlobalUnicast() {
				return nil, fmt.Errorf("dialing to non-local ip %v is prohibited", ip)
			}
			for _, subnet := range privateSubnets {
				if subnet.Contains(ip) {
					return nil, fmt.Errorf("dialing to ip %v from subnet %v is prohibited", ip, subnet)
				}
			}
		}
		return dialer.Dial(network, address)
	}
	var tr http.Transport
	tr = *(http.DefaultTransport).(*http.Transport)
	tr.Dial = dialFunc
	return &tr, nil
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
