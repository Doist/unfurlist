// Command unfurlist implements http server exposing API endpoint
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"bitbucket.org/doist/unfurlist"
	"github.com/bradfitz/gomemcache/memcache"
)

func main() {
	var (
		listen            = "127.0.0.1:8080"
		cache             = ""
		certfile, keyfile string
	)
	flag.StringVar(&listen, "listen", listen, "`address` to listen, set both -sslcert and -sslkey for HTTPS")
	flag.StringVar(&certfile, "sslcert", "", "path to certificate `file` (PEM)")
	flag.StringVar(&keyfile, "sslkey", "", "path to certificate key `file` (PEM)")
	flag.StringVar(&cache, "cache", cache, "`address` to memcached client (both host and ip)")
	flag.Parse()

	// Log
	log := log.New(os.Stderr, "", log.LstdFlags)

	// Memcache
	var mc *memcache.Client

	if cache != "" {
		log.Print("Setting up cache")
		mc = memcache.New(cache)
	}

	config := unfurlist.UnfurlConfig{
		Log:   log,
		Cache: mc,
	}

	handler := unfurlist.New(&config)

	if certfile != "" && keyfile != "" {
		log.Fatal(http.ListenAndServeTLS(listen, certfile, keyfile, handler))
	} else {
		log.Fatal(http.ListenAndServe(listen, handler))
	}
}
