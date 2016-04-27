// Command unfurlist implements http server exposing API endpoint
package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
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
		timeout           = 30 * time.Second
		withDimensions    bool
	)
	flag.DurationVar(&timeout, "timeout", timeout, "timeout for remote i/o")
	flag.StringVar(&listen, "listen", listen, "`address` to listen, set both -sslcert and -sslkey for HTTPS")
	flag.StringVar(&pprofListen, "pprof", pprofListen, "`address` to serve pprof data at (usually localhost)")
	flag.StringVar(&certfile, "sslcert", "", "path to certificate `file` (PEM)")
	flag.StringVar(&keyfile, "sslkey", "", "path to certificate key `file` (PEM)")
	flag.StringVar(&cache, "cache", cache, "`address` of memcached, if unset, caching is not used")
	flag.StringVar(&blacklist, "blacklist", blacklist, "path to `file` with url prefixes to blacklist, one per line")
	flag.BoolVar(&withDimensions, "withDimensions", withDimensions, "return image dimensions in result where possible (extra external request to fetch image)")
	flag.Parse()

	if timeout < 0 {
		timeout = 0
	}
	config := unfurlist.Config{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Log:            log.New(os.Stderr, "", log.LstdFlags),
		FetchImageSize: withDimensions,
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

	handler := unfurlist.New(&config)
	if pprofListen != "" {
		go func(addr string) { log.Println(http.ListenAndServe(addr, nil)) }(pprofListen)
	}
	go func() {
		// on a highly used system unfurlist can accumulate a lot of
		// idle connections occupying memory; force periodic close of
		// them
		for range time.NewTicker(2 * time.Minute).C {
			http.DefaultTransport.(*http.Transport).CloseIdleConnections()
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
