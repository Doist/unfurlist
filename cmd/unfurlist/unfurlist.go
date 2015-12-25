// Command unfurlist implements http server exposing API endpoint
package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"bitbucket.org/doist/unfurlist"
	"github.com/bradfitz/gomemcache/memcache"
)

func main() {
	var (
		listen            = "127.0.0.1:8080"
		pprofListen       = "127.0.0.1:6060"
		cache             = ""
		certfile, keyfile string
		timeout           = 30 * time.Second
	)
	flag.DurationVar(&timeout, "timeout", timeout, "timeout for remote i/o")
	flag.StringVar(&listen, "listen", listen, "`address` to listen, set both -sslcert and -sslkey for HTTPS")
	flag.StringVar(&pprofListen, "pprof", pprofListen, "`address` to serve pprof data at (usually localhost)")
	flag.StringVar(&certfile, "sslcert", "", "path to certificate `file` (PEM)")
	flag.StringVar(&keyfile, "sslkey", "", "path to certificate key `file` (PEM)")
	flag.StringVar(&cache, "cache", cache, "`address` to memcached client (both host and ip)")
	flag.Parse()

	if timeout < 0 {
		timeout = 0
	}
	config := unfurlist.UnfurlConfig{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Log: log.New(os.Stderr, "", log.LstdFlags),
	}
	if cache != "" {
		log.Print("Enable cache at ", cache)
		config.Cache = memcache.New(cache)
	}

	handler := unfurlist.New(&config)
	if pprofListen != "" {
		go func(addr string) { log.Println(http.ListenAndServe(addr, nil)) }(pprofListen)
	}

	if certfile != "" && keyfile != "" {
		log.Fatal(http.ListenAndServeTLS(listen, certfile, keyfile, handler))
	} else {
		log.Fatal(http.ListenAndServe(listen, handler))
	}
}
